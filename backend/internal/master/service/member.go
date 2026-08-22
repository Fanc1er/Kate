package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// MemberService 用户管理（邀请/移除/改角色/启停，单租户）。
type MemberService struct {
	DB    *gorm.DB
	Audit *AuditWriter
	Mail  *MailService
}

// NewMemberService 构造 MemberService。
func NewMemberService(db *gorm.DB, audit *AuditWriter, mail *MailService) *MemberService {
	return &MemberService{DB: db, Audit: audit, Mail: mail}
}

// List 用户列表。
func (s *MemberService) List(page, pageSize int) ([]map[string]any, int64, error) {
	var total int64
	s.DB.Model(&models.User{}).Count(&total)
	var users []models.User
	if err := s.DB.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{
			"user_id":       u.ID,
			"username":      u.Username,
			"email":         u.Email,
			"phone":         u.Phone,
			"role":          u.Role,
			"status":        u.Status,
			"last_login_at": u.LastLoginAt,
		})
	}
	return out, total, nil
}

// Invite 邀请成员（按邮箱匹配已有用户，否则创建 invited 用户并发邀请邮件）。
func (s *MemberService) Invite(email, role string, operatorID int64, operatorName, ip, ua string) (*models.User, error) {
	if role == "" {
		role = models.RoleUser
	}
	if role != models.RoleAdmin && role != models.RoleUser {
		return nil, errs.New(errs.CodeValidationFailed, "角色非法")
	}
	var user models.User
	if err := s.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// 未注册用户：创建 invited 状态用户。
		user = models.User{Username: email, Email: email, Role: role, Status: models.StatusInvited}
		if err := s.DB.Create(&user).Error; err != nil {
			return nil, err
		}
		if s.Mail != nil {
			_ = s.Mail.Send(email, "CInsight 用户邀请", fmt.Sprintf("您已被邀请加入 CInsight（角色 %s），请注册账号后登录。", role))
		}
	} else if err := s.DB.Model(&user).Update("role", role).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(operatorID, operatorName, "member.invite", "member", fmt.Sprint(user.ID), "", email, ip, ua)
	}
	return &user, nil
}

// UpdateRole 修改用户角色。
func (s *MemberService) UpdateRole(userID int64, role string, operatorID int64, operatorName, ip, ua string) error {
	if role != models.RoleAdmin && role != models.RoleUser {
		return errs.New(errs.CodeValidationFailed, "角色非法")
	}
	var u models.User
	if err := s.DB.First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeNotFound, "用户不存在")
		}
		return err
	}
	if u.Role == models.RoleAdmin && role != models.RoleAdmin {
		// 不能降级最后一个 admin。
		var admins int64
		s.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&admins)
		if admins <= 1 {
			return errs.New(errs.CodeValidationFailed, "系统至少保留一名管理员")
		}
	}
	if err := s.DB.Model(&u).Update("role", role).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(operatorID, operatorName, "member.role", "member", fmt.Sprint(userID), u.Role, role, ip, ua)
	}
	return nil
}

// ToggleStatus 启停用户。
func (s *MemberService) ToggleStatus(userID int64, status string, operatorID int64, operatorName, ip, ua string) error {
	if status != models.StatusActive && status != models.StatusDisabled {
		return errs.New(errs.CodeValidationFailed, "状态非法")
	}
	var u models.User
	if err := s.DB.First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeNotFound, "用户不存在")
		}
		return err
	}
	if err := s.DB.Model(&u).Update("status", status).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(operatorID, operatorName, "member.status", "member", fmt.Sprint(userID), u.Status, status, ip, ua)
	}
	return nil
}

// Remove 删除用户（不能删除自己与最后一个 admin）。
func (s *MemberService) Remove(userID, operatorID int64, operatorName, ip, ua string) error {
	if userID == operatorID {
		return errs.New(errs.CodeValidationFailed, "不能删除自己")
	}
	var u models.User
	if err := s.DB.First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeNotFound, "用户不存在")
		}
		return err
	}
	if u.Role == models.RoleAdmin {
		var admins int64
		s.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&admins)
		if admins <= 1 {
			return errs.New(errs.CodeValidationFailed, "系统至少保留一名管理员")
		}
	}
	if err := s.DB.Delete(&models.User{}, userID).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(operatorID, operatorName, "member.remove", "member", fmt.Sprint(userID), "", "removed", ip, ua)
	}
	return nil
}
