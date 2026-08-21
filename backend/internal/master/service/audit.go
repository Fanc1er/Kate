package service

import (
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/masker"
)

// AuditWriter 审计日志写入器（audit_logs 仅 insert，禁止修改删除）。
type AuditWriter struct {
	DB *gorm.DB
}

// NewAuditWriter 构造 AuditWriter。
func NewAuditWriter(db *gorm.DB) *AuditWriter {
	return &AuditWriter{DB: db}
}

// Write 写入一条审计记录（before/after 含手机号/邮箱/身份证/密钥时脱敏后落库）。
func (w *AuditWriter) Write(orgID, userID int64, username, action, resourceType, resourceID, before, after, ip, ua string) {
	if w == nil || w.DB == nil {
		return
	}
	rec := &models.AuditLog{
		OrgID:        orgID,
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		BeforeValue:  masker.MaskString(before),
		AfterValue:   masker.MaskString(after),
		IP:           ip,
		UserAgent:    masker.MaskString(ua),
	}
	w.DB.Create(rec)
}
