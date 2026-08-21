package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// PolicyService 策略模板（组织可复制平台默认模板，org_id=0 为平台级只读）。
type PolicyService struct {
	DB    *gorm.DB
	Audit *AuditWriter
}

// PolicyInput 策略模板输入。
type PolicyInput struct {
	Name             string `json:"name"`
	Scenario         string `json:"scenario"`
	EngineSwitches   string `json:"engine_switches"`
	Concurrency      int    `json:"concurrency"`
	Timeout          int    `json:"timeout"`
	RateLimit        int    `json:"rate_limit"`
	ScanDepth        int    `json:"scan_depth"`
	ConcurrencyLimit int    `json:"concurrency_limit"`
	AllowStatic      bool   `json:"allow_static"`
	SameOrigin       bool   `json:"same_origin"`
	CrawlSubpages    bool   `json:"crawl_subpages"`
}

// ToModel 转换输入为模型。
func (p PolicyInput) ToModel() *models.ScanPolicy {
	if p.Scenario == "" {
		p.Scenario = "daily"
	}
	if p.EngineSwitches == "" {
		p.EngineSwitches = `{}`
	}
	return &models.ScanPolicy{
		Name: p.Name, Scenario: p.Scenario, EngineSwitches: p.EngineSwitches,
		Concurrency: p.Concurrency, Timeout: p.Timeout, RateLimit: p.RateLimit,
		ScanDepth: p.ScanDepth, ConcurrencyLimit: p.ConcurrencyLimit,
		AllowStatic: p.AllowStatic, SameOrigin: p.SameOrigin, CrawlSubpages: p.CrawlSubpages,
	}
}

// NewPolicyService 构造 PolicyService。
func NewPolicyService(db *gorm.DB, audit *AuditWriter) *PolicyService {
	return &PolicyService{DB: db, Audit: audit}
}

// List 列出本组织可见模板（平台默认 + 组织自建）。
func (s *PolicyService) List(orgID int64) ([]models.ScanPolicy, error) {
	var list []models.ScanPolicy
	err := s.DB.Where("org_id IN (0, ?)", orgID).Order("org_id ASC, id ASC").Find(&list).Error
	return list, err
}

// Create 组织创建自定义模板。
func (s *PolicyService) Create(orgID int64, p *models.ScanPolicy, userID int64, username, ip, ua string) (*models.ScanPolicy, error) {
	if p.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "策略名称不能为空")
	}
	if p.Concurrency <= 0 {
		p.Concurrency = 4
	}
	if p.Timeout <= 0 {
		p.Timeout = 60
	}
	p.OrgID = orgID
	if p.EngineSwitches == "" {
		p.EngineSwitches = `{}`
	}
	if err := s.DB.Create(p).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "policy.create", "policy", fmt.Sprint(p.ID), "", p.Name, ip, ua)
	}
	return p, nil
}

// Update 更新组织自建模板。
func (s *PolicyService) Update(orgID, id int64, p *models.ScanPolicy, userID int64, username, ip, ua string) (*models.ScanPolicy, error) {
	var old models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&old).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "策略模板不存在")
		}
		return nil, err
	}
	p.ID = id
	p.OrgID = orgID
	if err := s.DB.Model(&old).Updates(map[string]any{
		"name": p.Name, "scenario": p.Scenario, "engine_switches": p.EngineSwitches,
		"concurrency": p.Concurrency, "timeout": p.Timeout, "rate_limit": p.RateLimit,
		"scan_depth": p.ScanDepth, "concurrency_limit": p.ConcurrencyLimit,
		"allow_static": p.AllowStatic, "same_origin": p.SameOrigin, "crawl_subpages": p.CrawlSubpages,
	}).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "policy.update", "policy", fmt.Sprint(id), "", p.Name, ip, ua)
	}
	var updated models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&updated).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete 删除组织自建模板。
func (s *PolicyService) Delete(orgID, id int64, userID int64, username, ip, ua string) error {
	var p models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeNotFound, "策略模板不存在")
		}
		return err
	}
	if err := s.DB.Delete(&models.ScanPolicy{}, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "policy.delete", "policy", fmt.Sprint(id), "", p.Name, ip, ua)
	}
	return nil
}

// CopyPlatform 复制平台默认模板到组织（TaskScheduler 创建计划时使用）。
func (s *PolicyService) CopyPlatform(orgID, platformID int64) (int64, error) {
	var src models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id = 0", platformID).First(&src).Error; err != nil {
		return 0, errs.New(errs.CodeNotFound, "平台默认策略不存在")
	}
	cp := src
	cp.ID = 0
	cp.OrgID = orgID
	cp.Name = src.Name + " (副本)"
	cp.Version = 1
	cp.CreatedAt = time.Time{}
	cp.UpdatedAt = time.Time{}
	if err := s.DB.Create(&cp).Error; err != nil {
		return 0, err
	}
	return cp.ID, nil
}

// Get 获取策略（含平台级）。
func (s *PolicyService) Get(orgID, id int64) (*models.ScanPolicy, error) {
	var p models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id IN (0, ?)", id, orgID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// EngineEnabled 检查引擎开关。
func (s *PolicyService) EngineEnabled(p *models.ScanPolicy, engine string) bool {
	var m map[string]any
	if err := json.Unmarshal([]byte(p.EngineSwitches), &m); err != nil {
		return true
	}
	v, ok := m[engine]
	if !ok {
		return true
	}
	return v == true || v == "true"
}
