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

func seedUser(t *testing.T, s *AuthService, username, pwd, email, role, status string) *models.User {
	t.Helper()
	hash, err := utils.HashPassword(pwd)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{Username: username, Password: hash, Email: email, Role: role, Status: status}
	if err := s.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return &u
}

func TestLoginSuccess(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", models.RoleUser, models.StatusActive)

	res, err := s.Login("alice", "Test@123456aA!", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.User == nil || res.User.Role != models.RoleUser {
		t.Fatalf("user = %+v", res.User)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("token 不应为空")
	}
	// access token 携带当前用户 role。
	claims, err := s.Tokens.Parse(res.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.UserID != u.ID || claims.Role != models.RoleUser {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "bob", "Test@123456aA!", "bob@x.com", models.RoleUser, models.StatusActive)
	_, err := s.Login("bob", "WrongPass123!", "127.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeAuthFailed {
		t.Fatalf("错误密码应返回 CodeAuthFailed, got %v", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "carol", "Test@123456aA!", "carol@x.com", models.RoleUser, models.StatusDisabled)
	_, err := s.Login("carol", "Test@123456aA!", "127.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeUserDisabled {
		t.Fatalf("禁用用户应返回 CodeUserDisabled, got %v", err)
	}
}

func TestLoginLockout(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "dave", "Test@123456aA!", "dave@x.com", models.RoleUser, models.StatusActive)

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
	seedUser(t, s, "eve", "Test@123456aA!", "eve@x.com", models.RoleUser, models.StatusActive)
	// 同一 IP 连续请求超过 5 次/min 触发限流。
	for i := 0; i < 6; i++ {
		_, _ = s.Login("eve", "WrongPass123!", "10.0.0.1")
	}
	_, err := s.Login("eve", "Test@123456aA!", "10.0.0.1")
	if err == nil || errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("IP 限流应返回 ValidationFailed, got %v", err)
	}
}

func TestLoginAdmin(t *testing.T) {
	s := newAuthService(t)
	seedUser(t, s, "root", "Test@123456aA!", "root@x.com", models.RoleAdmin, models.StatusActive)
	res, err := s.Login("root", "Test@123456aA!", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.User == nil || res.User.Role != models.RoleAdmin {
		t.Fatalf("admin user = %+v", res.User)
	}
	claims, err := s.Tokens.Parse(res.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.Role != models.RoleAdmin {
		t.Fatalf("access token 应带 admin 角色, claims=%+v", claims)
	}
}

func TestMe(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", models.RoleUser, models.StatusActive)

	me, err := s.Me(u.ID)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.Role != models.RoleUser {
		t.Fatalf("me = %+v", me)
	}
	if len(me.Permissions) == 0 {
		t.Fatal("me 应返回权限码集")
	}
}

func TestChangePasswordInvalidatesTokens(t *testing.T) {
	s := newAuthService(t)
	u := seedUser(t, s, "alice", "Test@123456aA!", "alice@x.com", models.RoleUser, models.StatusActive)

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
