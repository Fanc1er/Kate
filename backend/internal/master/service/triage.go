package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/repository"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// TriageService 处置面：findings/events/alerts/vulnerabilities。
type TriageService struct {
	DB      *gorm.DB
	Audit   *AuditWriter
	Evidence *EvidenceService
}

// NewTriageService 构造 TriageService。
func NewTriageService(db *gorm.DB, audit *AuditWriter, evidence *EvidenceService) *TriageService {
	return &TriageService{DB: db, Audit: audit, Evidence: evidence}
}

// guard 返回组织隔离守卫（org_id 强制过滤，缺省 org_id 拒绝查询）。
func (s *TriageService) guard(orgID int64) *repository.Guard {
	g, err := repository.NewGuard(s.DB, orgID)
	if err != nil {
		panic(err)
	}
	return g
}

// ---------- Findings ----------

// FindingListParams 查询参数。
type FindingListParams struct {
	Status     string
	Severity   string
	EngineName string
	Type       string
	AssetID    int64
	TaskID     int64
	Keyword    string
	Page       int
	PageSize   int
}

// ListFindings 查询 findings。
func (s *TriageService) ListFindings(orgID int64, p FindingListParams) ([]models.Finding, int64, error) {
	q := s.guard(orgID).Scoped(&models.Finding{})
	if p.Status != "" {
		q = q.Where("status = ?", p.Status)
	}
	if p.Severity != "" {
		q = q.Where("severity = ?", p.Severity)
	}
	if p.EngineName != "" {
		q = q.Where("engine_name = ?", p.EngineName)
	}
	if p.Type != "" {
		q = q.Where("type = ?", p.Type)
	}
	if p.AssetID > 0 {
		q = q.Where("asset_id = ?", p.AssetID)
	}
	if p.TaskID > 0 {
		q = q.Where("task_id = ?", p.TaskID)
	}
	if p.Keyword != "" {
		q = q.Where("title LIKE ? OR url LIKE ?", "%"+p.Keyword+"%", "%"+p.Keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Finding
	if err := q.Order("risk_score DESC, id DESC").Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateFindingStatus 更新 finding 状态并联动关闭漏洞/告警。
func (s *TriageService) UpdateFindingStatus(orgID, id int64, status string, userID int64, username, ip, ua string) (*models.Finding, error) {
	valid := map[string]bool{"open": true, "closed": true, "confirmed": true, "ignored": true}
	if !valid[status] {
		return nil, errs.New(errs.CodeValidationFailed, "非法状态")
	}
	var f models.Finding
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&f).Error; err != nil {
		return nil, err
	}
	if err := s.DB.Model(&f).Update("status", status).Error; err != nil {
		return nil, err
	}
	if status == "closed" {
		now := time.Now()
		s.DB.Model(&models.Alert{}).Where("finding_id = ? AND org_id = ?", id, orgID).
			Updates(map[string]any{"status": "resolved", "resolved_at": now})
		s.DB.Model(&models.Vulnerability{}).Where("finding_id = ? AND org_id = ?", id, orgID).
			Updates(map[string]any{"status": "closed", "closed_at": now})
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "finding.update", "finding", fmt.Sprint(id), f.Status, status, ip, ua)
	}
	return &f, nil
}

// GetFindingDetail finding 详情 + 证据链。
func (s *TriageService) GetFindingDetail(orgID, id int64) (map[string]any, error) {
	var f models.Finding
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&f).Error; err != nil {
		return nil, err
	}
	var evs []models.Evidence
	var evIDs []int64
	_ = json.Unmarshal([]byte(f.EvidenceIDs), &evIDs)
	if len(evIDs) > 0 {
		s.DB.Where("id IN ? AND org_id = ?", evIDs, orgID).Find(&evs)
	}
	detail := map[string]any{"finding": f, "evidence": evs}
	// type=sensitive_info 返回命中明细（sensitive_info_hits 表按 task+url 关联）。
	if f.Type == "sensitive_info" {
		var hits []models.SensitiveInfoHit
		s.DB.Where("org_id = ? AND task_id = ? AND url = ?", orgID, f.TaskID, f.URL).
			Order("id DESC").Limit(50).Find(&hits)
		detail["sensitive_info_hits"] = hits
	}
	return detail, nil
}

// ---------- Events ----------

// ListEvents 事件列表。
func (s *TriageService) ListEvents(orgID int64, status, eventType string, page, pageSize int) ([]models.Event, int64, error) {
	q := s.guard(orgID).Scoped(&models.Event{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Event
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateEventStatus 事件状态流转 pending→handling→resolved。
func (s *TriageService) UpdateEventStatus(orgID, id int64, status string, userID int64, username, ip, ua string) error {
	valid := map[string]bool{"pending": true, "handling": true, "resolved": true}
	if !valid[status] {
		return errs.New(errs.CodeValidationFailed, "非法状态")
	}
	var ev models.Event
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&ev).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&ev).Update("status", status).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "event.update", "event", fmt.Sprint(id), ev.Status, status, ip, ua)
	}
	return nil
}

// ---------- Alerts ----------

// ListAlerts 告警列表。
func (s *TriageService) ListAlerts(orgID int64, status, alertType string, page, pageSize int) ([]models.Alert, int64, error) {
	q := s.guard(orgID).Scoped(&models.Alert{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if alertType != "" {
		q = q.Where("alert_type = ?", alertType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Alert
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ResolveAlert 关闭告警。
func (s *TriageService) ResolveAlert(orgID, id int64, userID int64, username, ip, ua string) error {
	var a models.Alert
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&a).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := s.DB.Model(&a).Updates(map[string]any{"status": "resolved", "resolved_at": now}).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "alert.resolve", "alert", fmt.Sprint(id), a.Status, "resolved", ip, ua)
	}
	return nil
}

// ---------- Vulnerabilities ----------

// ListVulns 漏洞列表。
func (s *TriageService) ListVulns(orgID int64, status, severity string, assetID int64, page, pageSize int) ([]models.Vulnerability, int64, error) {
	q := s.guard(orgID).Scoped(&models.Vulnerability{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if assetID > 0 {
		q = q.Where("asset_id = ?", assetID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Vulnerability
	if err := q.Order("severity = 'critical' DESC, severity = 'high' DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetVulnDetail 漏洞详情 + 证据链。
func (s *TriageService) GetVulnDetail(orgID, id int64) (map[string]any, error) {
	var v models.Vulnerability
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&v).Error; err != nil {
		return nil, err
	}
	var evs []models.Evidence
	var evIDs []int64
	_ = json.Unmarshal([]byte(v.EvidenceIDs), &evIDs)
	if len(evIDs) > 0 {
		s.DB.Where("id IN ? AND org_id = ?", evIDs, orgID).Find(&evs)
	}
	return map[string]any{"vulnerability": v, "evidence": evs}, nil
}

// GetVulnEvidence 漏洞关联证据链聚合（含证据文件与读取校验）。
func (s *TriageService) GetVulnEvidence(orgID, id int64) ([]map[string]any, error) {
	var v models.Vulnerability
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	var evIDs []int64
	_ = json.Unmarshal([]byte(v.EvidenceIDs), &evIDs)
	if len(evIDs) == 0 {
		return []map[string]any{}, nil
	}
	var evs []models.Evidence
	s.DB.Where("id IN ? AND org_id = ?", evIDs, orgID).Find(&evs)
	out := make([]map[string]any, 0, len(evs))
	for _, ev := range evs {
		files, err := s.Evidence.Files(orgID, ev.ID)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id":        ev.ID,
			"md5":       ev.MD5,
			"sha256":    ev.SHA256,
			"file_path": ev.FilePath,
			"mime_type": ev.MimeType,
			"size":      ev.Size,
			"created_at": ev.CreatedAt,
			"files":     files,
		})
	}
	return out, nil
}

// ---------- 事件/告警/漏洞统计 ----------

// TriageOverview 处置面板聚合统计。
func (s *TriageService) TriageOverview(orgID int64) (map[string]any, error) {
	type cnt struct {
		Status string
		Cnt    int64
	}
	agg := func(m *gorm.DB, prefix string) map[string]int64 {
		out := map[string]int64{}
		var rows []cnt
		m.Scan(&rows)
		for _, r := range rows {
			out[prefix+r.Status] = r.Cnt
		}
		return out
	}
	m := map[string]any{}
	var fRows, eRows, aRows, vRows []cnt
	s.DB.Model(&models.Finding{}).Select("status, COUNT(*) AS cnt").Where("org_id = ?", orgID).Group("status").Scan(&fRows)
	s.DB.Model(&models.Event{}).Select("status, COUNT(*) AS cnt").Where("org_id = ?", orgID).Group("status").Scan(&eRows)
	s.DB.Model(&models.Alert{}).Select("status, COUNT(*) AS cnt").Where("org_id = ?", orgID).Group("status").Scan(&aRows)
	s.DB.Model(&models.Vulnerability{}).Select("status, COUNT(*) AS cnt").Where("org_id = ?", orgID).Group("status").Scan(&vRows)
	for _, r := range fRows {
		m["findings_"+r.Status] = r.Cnt
	}
	for _, r := range eRows {
		m["events_"+r.Status] = r.Cnt
	}
	for _, r := range aRows {
		m["alerts_"+r.Status] = r.Cnt
	}
	for _, r := range vRows {
		m["vulns_"+r.Status] = r.Cnt
	}
	_ = agg
	// 今日新增。
	var todayNew int64
	s.DB.Model(&models.Alert{}).Where("org_id = ? AND created_at >= ?", orgID, time.Now().Truncate(24*time.Hour)).Count(&todayNew)
	m["alerts_today"] = todayNew
	return m, nil
}

// -------- 内部工具 --------

// ParseIDs 解析逗号分隔 ID 列表。
func ParseIDs(s string) []int64 {
	var ids []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := utils.ParseInt64(part); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

var _ = errors.Is
var _ = fmt.Sprint
