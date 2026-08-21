package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// MemberService 组织成员管理（邀请/移除/改角色/启停）。
type MemberService struct {
	DB    *gorm.DB
	Audit *AuditWriter
	Mail  *MailService
}

// NewMemberService 构造 MemberService。
func NewMemberService(db *gorm.DB, audit *AuditWriter, mail *MailService) *MemberService {
	return &MemberService{DB: db, Audit: audit, Mail: mail}
}

// List 成员列表。
func (s *MemberService) List(orgID int64, page, pageSize int) ([]map[string]any, int64, error) {
	var total int64
	s.DB.Model(&models.UserOrg{}).Where("org_id = ?", orgID).Count(&total)
	var uos []models.UserOrg
	if err := s.DB.Where("org_id = ?", orgID).Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&uos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]map[string]any, 0, len(uos))
	for _, uo := range uos {
		var u models.User
		m := map[string]any{"role": uo.Role, "status": uo.Status, "joined_at": uo.JoinedAt}
		if err := s.DB.First(&u, uo.UserID).Error; err == nil {
			m["user_id"] = u.ID
			m["username"] = u.Username
			m["email"] = u.Email
			m["phone"] = u.Phone
		}
		out = append(out, m)
	}
	return out, total, nil
}

// Invite 邀请成员（按邮箱匹配已有用户，否则发邀请邮件）。
func (s *MemberService) Invite(orgID int64, email, role string, operatorID int64, operatorName, ip, ua string) (*models.UserOrg, error) {
	if role == "" {
		role = models.RoleViewer
	}
	if role != models.RoleOrgAdmin && role != models.RoleEngineer && role != models.RoleViewer {
		return nil, errs.New(errs.CodeValidationFailed, "角色非法")
	}
	// 配额校验。
	var org models.Organization
	if err := s.DB.First(&org, orgID).Error; err != nil {
		return nil, err
	}
	var used int64
	s.DB.Model(&models.UserOrg{}).Where("org_id = ?", orgID).Count(&used)
	if int(used) >= org.MaxMembers {
		return nil, errs.New(errs.CodeMemberQuota, "")
	}
	var user models.User
	if err := s.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// 未注册用户：邀请邮件（MVP 记录 invited 状态）。
		user = models.User{Username: email, Email: email, Status: models.StatusInvited}
		if err := s.DB.Create(&user).Error; err != nil {
			return nil, err
		}
		if s.Mail != nil {
			_ = s.Mail.Send(email, "CInsight 组织邀请", fmt.Sprintf("您已被邀请加入组织 %s（角色 %s），请注册账号后登录。", org.Name, role))
		}
	} else if user.IsSuperAdmin {
		// super_admin 走平台通道（org_id=0），禁止加入业务组织。
		return nil, errs.New(errs.CodeValidationFailed, "超级管理员无需加入组织")
	}
	// 已存在成员关系则更新角色。
	var uo models.UserOrg
	if err := s.DB.Where("user_id = ? AND org_id = ?", user.ID, orgID).First(&uo).Error; err == nil {
		if err := s.DB.Model(&uo).Update("role", role).Error; err != nil {
			return nil, err
		}
	} else {
		uo = models.UserOrg{UserID: user.ID, OrgID: orgID, Role: role, Status: models.StatusActive, JoinedAt: time.Now()}
		if err := s.DB.Create(&uo).Error; err != nil {
			return nil, err
		}
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, operatorID, operatorName, "member.invite", "member", fmt.Sprint(user.ID), "", email, ip, ua)
	}
	return &uo, nil
}

// UpdateRole 修改成员角色。
func (s *MemberService) UpdateRole(orgID, userID int64, role string, operatorID int64, operatorName, ip, ua string) error {
	if role != models.RoleOrgAdmin && role != models.RoleEngineer && role != models.RoleViewer {
		return errs.New(errs.CodeValidationFailed, "角色非法")
	}
	var uo models.UserOrg
	if err := s.DB.Where("user_id = ? AND org_id = ?", userID, orgID).First(&uo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.New(errs.CodeNotFound, "成员不存在")
		}
		return err
	}
	if err := s.DB.Model(&uo).Update("role", role).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, operatorID, operatorName, "member.role", "member", fmt.Sprint(userID), uo.Role, role, ip, ua)
	}
	return nil
}

// ToggleStatus 启停成员。
func (s *MemberService) ToggleStatus(orgID, userID int64, status string, operatorID int64, operatorName, ip, ua string) error {
	if status != models.StatusActive && status != models.StatusDisabled {
		return errs.New(errs.CodeValidationFailed, "状态非法")
	}
	var uo models.UserOrg
	if err := s.DB.Where("user_id = ? AND org_id = ?", userID, orgID).First(&uo).Error; err != nil {
		return err
	}
	if err := s.DB.Model(&uo).Update("status", status).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, operatorID, operatorName, "member.status", "member", fmt.Sprint(userID), uo.Status, status, ip, ua)
	}
	return nil
}

// Remove 移除成员（不能移除自己与最后一个 admin）。
func (s *MemberService) Remove(orgID, userID, operatorID int64, operatorName, ip, ua string) error {
	if userID == operatorID {
		return errs.New(errs.CodeValidationFailed, "不能移除自己")
	}
	var uo models.UserOrg
	if err := s.DB.Where("user_id = ? AND org_id = ?", userID, orgID).First(&uo).Error; err != nil {
		return err
	}
	if uo.Role == models.RoleOrgAdmin {
		var admins int64
		s.DB.Model(&models.UserOrg{}).Where("org_id = ? AND role = ?", orgID, models.RoleOrgAdmin).Count(&admins)
		if admins <= 1 {
			return errs.New(errs.CodeValidationFailed, "组织至少保留一名管理员")
		}
	}
	if err := s.DB.Where("user_id = ? AND org_id = ?", userID, orgID).Delete(&models.UserOrg{}).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, operatorID, operatorName, "member.remove", "member", fmt.Sprint(userID), "", "removed", ip, ua)
	}
	return nil
}

// ---- 组织管理 ----

// CreateOrg 创建组织（首个组织管理员自动加入）。
func (s *MemberService) CreateOrg(ownerID int64, name, plan string, operatorName, ip, ua string) (*models.Organization, error) {
	if name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "组织名不能为空")
	}
	// super_admin 走平台通道（org_id=0），禁止创建业务组织。
	var owner models.User
	if err := s.DB.First(&owner, ownerID).Error; err != nil {
		return nil, err
	}
	if owner.IsSuperAdmin {
		return nil, errs.New(errs.CodeValidationFailed, "超级管理员无需创建组织")
	}
	if plan == "" {
		plan = "free"
	}
	org := &models.Organization{Name: name, Plan: plan, Status: models.StatusActive}
	if err := s.DB.Create(org).Error; err != nil {
		return nil, err
	}
	uo := &models.UserOrg{UserID: ownerID, OrgID: org.ID, Role: models.RoleOrgAdmin, Status: models.StatusActive, JoinedAt: time.Now()}
	if err := s.DB.Create(uo).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(org.ID, ownerID, operatorName, "org.create", "org", fmt.Sprint(org.ID), "", name, ip, ua)
	}
	return org, nil
}

// ListMyOrgs 我的组织列表（供超管以外用户）。
func (s *MemberService) ListMyOrgs(userID int64) ([]models.Organization, error) {
	var uos []models.UserOrg
	if err := s.DB.Where("user_id = ? AND status = ?", userID, models.StatusActive).Find(&uos).Error; err != nil {
		return nil, err
	}
	var orgs []models.Organization
	for _, uo := range uos {
		var org models.Organization
		if err := s.DB.First(&org, uo.OrgID).Error; err == nil {
			orgs = append(orgs, org)
		}
	}
	return orgs, nil
}

var _ = utils.MD5Hex
