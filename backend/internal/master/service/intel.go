package service

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// IntelService 组件漏洞情报管理。
// 条目约定：TechStack 存组件名（nginx/apache/tomcat/openresty/iis/jetty 等，
// 与引擎 detectComponent 输出一致），Scope 存受影响的最高版本号；
// 满足两字段的条目会随任务 component_rules 下发给情报引擎做版本匹配。
type IntelService struct {
	DB    *gorm.DB
	Audit *AuditWriter
}

// NewIntelService 构造情报服务。
func NewIntelService(db *gorm.DB, audit *AuditWriter) *IntelService {
	return &IntelService{DB: db, Audit: audit}
}

// IntelInput 手动导入的情报条目。
type IntelInput struct {
	IntelID     string `json:"intel_id"` // CVE-YYYY-NNNN / CNVD-... / CNNVD-...
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`    // critical/high/medium/low
	Component   string `json:"component"`   // 组件名，空则仅入库展示
	MaxVersion  string `json:"max_version"` // 受影响最高版本
}

// validate 校验必填字段与合法取值。
func (i *IntelInput) validate() error {
	if i.IntelID == "" || i.Title == "" {
		return errs.New(errs.CodeValidationFailed, "intel_id 与 title 必填")
	}
	switch i.Severity {
	case "critical", "high", "medium", "low", "":
	default:
		return errs.New(errs.CodeValidationFailed, "severity 非法")
	}
	return nil
}

// List 分页查询情报条目。
func (s *IntelService) List(page, pageSize int) ([]models.IntelItem, int64, error) {
	var total int64
	if err := s.DB.Model(&models.IntelItem{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.IntelItem
	err := s.DB.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// Import 批量导入情报，按 intel_id 幂等覆盖更新，返回写入条数。
func (s *IntelService) Import(inputs []IntelInput) (int64, error) {
	if len(inputs) == 0 {
		return 0, nil
	}
	items := make([]models.IntelItem, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		if err := in.validate(); err != nil {
			return 0, fmt.Errorf("第 %d 条: %w", i+1, err)
		}
		sev := in.Severity
		if sev == "" {
			sev = "high"
		}
		items = append(items, models.IntelItem{
			Source:    "manual",
			IntelID:   in.IntelID,
			Title:     in.Title,
			Severity:  sev,
			Scope:     in.MaxVersion,
			TechStack: in.Component,
		})
	}
	res := s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "intel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "severity", "scope", "tech_stack", "updated_at"}),
	}).Create(&items)
	return res.RowsAffected, res.Error
}

// Delete 删除情报条目。
func (s *IntelService) Delete(id int64, userID int64, username, ip, ua string) error {
	var item models.IntelItem
	if err := s.DB.Where("id = ?", id).First(&item).Error; err != nil {
		return err
	}
	if err := s.DB.Delete(&item).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(userID, username, "intel.delete", "intel_item", fmt.Sprint(id), item.IntelID, "", ip, ua)
	}
	return nil
}

// LoadComponentRules 查询可下发引擎的组件规则（有组件名与版本上限的条目）。
// WorkerService 构建任务载荷时直接以 DB 调用，避免装配依赖。
func LoadComponentRules(db *gorm.DB, limit int) []map[string]string {
	var items []models.IntelItem
	db.Where("tech_stack <> '' AND scope <> ''").Order("id DESC").Limit(limit).Find(&items)
	out := make([]map[string]string, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]string{
			"cve_id":      it.IntelID,
			"title":       it.Title,
			"severity":    it.Severity,
			"component":   it.TechStack,
			"max_version": it.Scope,
		})
	}
	return out
}
