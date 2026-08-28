package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// NotifyService 通知渠道管理：webhook 渠道 CRUD 与告警推送。
type NotifyService struct {
	DB    *gorm.DB
	Audit *AuditWriter
}

// NewNotifyService 构造 NotifyService。
func NewNotifyService(db *gorm.DB, audit *AuditWriter) *NotifyService {
	return &NotifyService{DB: db, Audit: audit}
}

// webhookClient 推送超时，避免告警路径被慢端点阻塞。
var webhookClient = &http.Client{Timeout: 5 * time.Second}

// webhookConfig 渠道配置。
type webhookConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

// List 列出全部通知渠道。
func (s *NotifyService) List() ([]models.NotifyChannel, error) {
	var list []models.NotifyChannel
	if err := s.DB.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新建通知渠道（当前仅支持 webhook 类型）。
func (s *NotifyService) Create(channelType, config string) (*models.NotifyChannel, error) {
	if channelType != "webhook" {
		return nil, errs.New(errs.CodeValidationFailed, "暂仅支持 webhook 类型渠道")
	}
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil || cfg.URL == "" {
		return nil, errs.New(errs.CodeValidationFailed, "config 需为 JSON 且 url 必填")
	}
	ch := &models.NotifyChannel{Type: channelType, Config: config, Enabled: "true"}
	if err := s.DB.Create(ch).Error; err != nil {
		return nil, err
	}
	return ch, nil
}

// Update 更新渠道配置/启用状态。
func (s *NotifyService) Update(id int64, config *string, enabled *string) (*models.NotifyChannel, error) {
	var ch models.NotifyChannel
	if err := s.DB.Where("id = ?", id).First(&ch).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if config != nil {
		var cfg webhookConfig
		if err := json.Unmarshal([]byte(*config), &cfg); err != nil || cfg.URL == "" {
			return nil, errs.New(errs.CodeValidationFailed, "config 需为 JSON 且 url 必填")
		}
		updates["config"] = *config
	}
	if enabled != nil && (*enabled == "true" || *enabled == "false") {
		updates["enabled"] = *enabled
	}
	if len(updates) > 0 {
		if err := s.DB.Model(&ch).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &ch, nil
}

// Delete 删除渠道。
func (s *NotifyService) Delete(id int64) error {
	return s.DB.Delete(&models.NotifyChannel{}, "id = ?", id).Error
}

// Test 向渠道发送一条测试消息。
func (s *NotifyService) Test(id int64) error {
	var ch models.NotifyChannel
	if err := s.DB.Where("id = ?", id).First(&ch).Error; err != nil {
		return err
	}
	payload := map[string]any{
		"event": "notify.test",
		"text":  "CInsight 通知渠道测试",
		"ts":    time.Now().Format(time.RFC3339),
	}
	return postWebhook(&ch, payload)
}

// PushAlert 向全部启用的 webhook 渠道推送告警；失败仅记录日志，不阻塞告警主流程。
func PushAlert(db *gorm.DB, alert *models.Alert, assetURL string) {
	if alert == nil {
		return
	}
	var channels []models.NotifyChannel
	db.Where("type = ? AND enabled = ?", "webhook", "true").Find(&channels)
	if len(channels) == 0 {
		return
	}
	payload := map[string]any{
		"event": "alert.new",
		"alert": map[string]any{
			"id": alert.ID, "asset_id": alert.AssetID, "finding_id": alert.FindingID,
			"type": alert.AlertType, "severity": alert.Severity,
			"title": alert.Title, "content": alert.Content,
			"url": assetURL, "created_at": alert.CreatedAt,
		},
	}
	for _, ch := range channels {
		if err := postWebhook(&ch, payload); err != nil {
			log.Printf("notify: webhook channel %d push failed: %v", ch.ID, err)
		}
	}
}

// postWebhook 发送 JSON payload；配置了 secret 时附带 X-CInsight-Secret 头供接收端校验。
func postWebhook(ch *models.NotifyChannel, payload map[string]any) error {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil || cfg.URL == "" {
		return fmt.Errorf("channel %d invalid config", ch.ID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Secret != "" {
		req.Header.Set("X-CInsight-Secret", cfg.Secret)
	}
	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook %d status %d", ch.ID, resp.StatusCode)
	}
	return nil
}
