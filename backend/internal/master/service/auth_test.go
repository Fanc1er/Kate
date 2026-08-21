package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func newAuthService(t *testing.T) *AuthService {
	t.Helper()
	gdb := newTestDB(t)
	tm := NewTokenManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	mail := NewMailService("", 0, "", "", "", 5*time.Minute)
	return NewAuthService(gdb, tm, mail)
}

func seedUser(t *testing.T, s *AuthService, username, pwd, email string, isSuper bool, status string) *models.User {
	t.Helper()
	hash, err := utils.HashPassword(pwd)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{Username: username, Password: hash, Email: email, IsSuperAdmin: isSuper, Status: status}
	if err := s.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return &u
}

func seedOrgMember(t *testing.T, s *AuthService, userID int64, role string, status string) int64 {
	t.Helper()
	org := models.Organization{Name: "org", Status: "active"}
	if err := s.DB.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	uo := models.UserOrg{UserID: userID, OrgID: org.ID, Role: role, Status: status, JoinedAt: time.Now()}
	if err := s.DB.Create(&uo).Error; err != nil {
		t.Fatalf("create user_org: %v", err)
	}
	return org.ID
}

func TestLoginSuccessSingleOrg(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", false, models.StatusActive)
	orgID := seedOrgMember(t, s, u.ID, models.RoleOrgAdmin, models.StatusActive)
	_ = orgID

	res, err := s.Login("alice", "Test@123456aA!", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.NeedSelectOrg {
		t.Fatal("单组织不应 need_select_org")
	}
	if res.User == nil || res.User.Role != models.RoleOrgAdmin {
		t.Fatalf("user = %+v", res.User)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("token 不应为空")
	}
	// 单组织直接换取带 org 的 access token。
	claims, err := s.Tokens.Parse(res.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.OrgID <= 0 {
		t.Fatalf("access token 应带 org_id, claims=%+v", claims)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "bob", "Test@123456aA!", "bob@x.com", false, models.StatusActive)
	_, err := s.Login("bob", "WrongPass123!", "127.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeAuthFailed {
		t.Fatalf("错误密码应返回 CodeAuthFailed, got %v", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "carol", "Test@123456aA!", "carol@x.com", false, models.StatusDisabled)
	_, err := s.Login("carol", "Test@123456aA!", "127.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeUserDisabled {
		t.Fatalf("禁用用户应返回 CodeUserDisabled, got %v", err)
	}
}

func TestLoginLockout(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "dave", "Test@123456aA!", "dave@x.com", false, models.StatusActive)

	// 连续失败 LoginMaxAttempts 次（每次换 IP，避免触发 IP 限流而非账户锁定）。
	for i := 0; i < LoginMaxAttempts; i++ {
		ip := fmt.Sprintf("10.1.1.%d", i+1)
		if _, err := s.Login("dave", "WrongPass123!", ip); err == nil {
			t.Fatalf("第 %d 次错误密码应失败", i+1)
		}
	}
	// 第 6 次即使密码正确也应锁定。
	_, err := s.Login("dave", "Test@123456aA!", "10.9.9.9")
	if err == nil || errs.CodeOf(err) != errs.CodeAccountLocked {
		t.Fatalf("锁定后应返回 CodeAccountLocked, got %v", err)
	}
}

func TestLoginIPRateLimit(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "eve", "Test@123456aA!", "eve@x.com", false, models.StatusActive)
	// 同一 IP 连续请求超过 5 次/min 触发限流。
	for i := 0; i < 6; i++ {
		_, _ = s.Login("eve", "WrongPass123!", "10.0.0.1")
	}
	_, err := s.Login("eve", "Test@123456aA!", "10.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("IP 限流应返回 ValidationFailed, got %v", err)
	}
}

func TestLoginSuperAdmin(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "root", "Test@123456aA!", "root@x.com", true, models.StatusActive)
	res, err := s.Login("root", "Test@123456aA!", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !res.IsSuperAdmin {
		t.Fatal("应为超管")
	}
	if res.User == nil || res.User.Role != models.RoleSuperAdmin {
		t.Fatalf("超管 user = %+v", res.User)
	}
	if res.NeedSelectOrg {
		t.Fatal("超管不应 need_select_org")
	}
}

func TestMe(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", false, models.StatusActive)
	orgID := seedOrgMember(t, s, u.ID, models.RoleEngineer, models.StatusActive)

	me, err := s.Me(u.ID)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Role != models.RoleEngineer || me.OrgID != orgID || me.OrgName != "org" {
		t.Fatalf("me = %+v", me)
	}
	if len(me.Permissions) == 0 {
		t.Fatal("me 应返回权限码集")
	}
}

func TestSelectOrg(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", false, models.StatusActive)
	orgID := seedOrgMember(t, s, u.ID, models.RoleOrgAdmin, models.StatusActive)

	res, err := s.SelectOrg(u.ID, orgID)
	if err != nil {
		t.Fatalf("SelectOrg: %v", err)
	}
	claims, err := s.Tokens.Parse(res.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.OrgID != orgID || claims.Role != models.RoleOrgAdmin {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestSelectOrgNotMember(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", false, models.StatusActive)
	org := models.Organization{Name: "Acme", Status: "active"}
	if err := s.DB.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := s.SelectOrg(u.ID, org.ID); err == nil {
		t.Fatal("非成员选择组织应失败")
	}
}

func TestChangePasswordInvalidatesTokens(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", false, models.StatusActive)
	seedOrgMember(t, s, u.ID, models.RoleEngineer, models.StatusActive)

	if err := s.ChangePassword(u.ID, "Test@123456aA!", "NewPass@123abc"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	// 旧密码失效。
	_, err := s.Login("alice", "Test@123456aA!", "127.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeAuthFailed {
		t.Fatalf("改密后旧密码应失效, got %v", err)
	}
	// 新密码可登录。
	if _, err := s.Login("alice", "NewPass@123abc", "127.0.0.1"); err != nil {
		t.Fatalf("改密后新密码应可登录: %v", err)
	}
}

func TestValidatePassword(t *testing.T) {
	valid := []string{"Test@123456aA!", "Abcdef@123456", "aB3!xYz98765"}
	invalid := []string{"short", "ALLUPPERCASE123!", "alllowercase123!", "NoSpecial12345", "NoDigit!@abcdef"}
	for _, p := range valid {
		if err := validatePassword(p); err != nil {
			t.Fatalf("密码 %q 应通过: %v", p, err)
		}
	}
	for _, p := range invalid {
		if err := validatePassword(p); err == nil {
			t.Fatalf("密码 %q 不应通过", p)
		}
	}
}
