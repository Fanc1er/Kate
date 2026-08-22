package service

import (
	"testing"
	"time"
)

func TestTokenIssueAndParse(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	pair, err := tm.Issue(42)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("token 不应为空")
	}
	if pair.ExpiresIn != 15*60 {
		t.Fatalf("ExpiresIn = %d, want 900", pair.ExpiresIn)
	}

	claims, err := tm.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("Parse access: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Fatalf("token type = %q, want access", claims.TokenType)
	}

	rc, err := tm.ParseRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ParseRefresh: %v", err)
	}
	if rc.UserID != 42 || rc.TokenType != TokenTypeRefresh {
		t.Fatal("refresh claims 不正确")
	}
}

func TestTokenWrongSecret(t *testing.T) {
	tm := NewTokenManager("secret-a", time.Minute, time.Hour)
	other := NewTokenManager("secret-b", time.Minute, time.Hour)
	pair, _ := tm.Issue(1)
	if _, err := other.Parse(pair.AccessToken); err == nil {
		t.Fatal("不同密钥应校验失败")
	}
}

func TestRefreshRotation(t *testing.T) {
	tm := NewTokenManager("secret", time.Minute, time.Hour)
	pair, _ := tm.Issue(7)

	// 模拟 AuthService.Refresh 语义：校验 refresh → 旧 jti 拉黑 → 换发新对。
	if _, err := tm.ParseRefresh(pair.RefreshToken); err != nil {
		t.Fatalf("旧 refresh 应可用: %v", err)
	}
	claims, err := tm.ParseRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ParseRefresh: %v", err)
	}
	tm.RevokeJTI(claims.ID, time.Hour)
	np, err := tm.Issue(claims.UserID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if np.AccessToken == pair.AccessToken {
		t.Fatal("换发后 access token 不应相同")
	}
	// 旧 refresh 已入黑名单。
	if _, err := tm.ParseRefresh(pair.RefreshToken); err == nil {
		t.Fatal("换发后旧 refresh token 应失效")
	}
	// 新 refresh 可用。
	if _, err := tm.ParseRefresh(np.RefreshToken); err != nil {
		t.Fatalf("新 refresh 应可用: %v", err)
	}
}

func TestRevokeJTI(t *testing.T) {
	tm := NewTokenManager("secret", time.Minute, time.Hour)
	pair, _ := tm.Issue(5)
	tm.RevokeJTI(parseRefreshJTI(t, tm, pair.RefreshToken), time.Hour)
	if _, err := tm.ParseRefresh(pair.RefreshToken); err == nil {
		t.Fatal("jti 拉黑后 refresh 应失效")
	}
}

func TestRevokeAllUserRefresh(t *testing.T) {
	tm := NewTokenManager("secret", time.Minute, time.Hour)
	p1, _ := tm.Issue(9)
	p2, _ := tm.Issue(9)
	tm.RevokeAllUserRefresh(9)
	if _, err := tm.ParseRefresh(p1.RefreshToken); err == nil {
		t.Fatal("全部 refresh 应失效")
	}
	if _, err := tm.ParseRefresh(p2.RefreshToken); err == nil {
		t.Fatal("全部 refresh 应失效")
	}
	// 其他用户不受影响。
	p3, _ := tm.Issue(10)
	if _, err := tm.ParseRefresh(p3.RefreshToken); err != nil {
		t.Fatalf("其他用户 refresh 不应受影响: %v", err)
	}
}

func TestExpiredToken(t *testing.T) {
	tm := NewTokenManager("secret", 0, time.Hour) // access 立即过期
	pair, _ := tm.Issue(1)
	if _, err := tm.Parse(pair.AccessToken); err == nil {
		t.Fatal("过期 token 应校验失败")
	}
}

func TestIssueWithRole(t *testing.T) {
	tm := NewTokenManager("secret", time.Minute, time.Hour)
	tok, err := tm.IssueWithRole(3, "admin")
	if err != nil {
		t.Fatalf("IssueWithRole: %v", err)
	}
	claims, err := tm.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.UserID != 3 || claims.Role != "admin" {
		t.Fatalf("claims = %+v", claims)
	}
}

func parseRefreshJTI(t *testing.T, tm *TokenManager, tok string) string {
	t.Helper()
	claims, err := tm.ParseRefresh(tok)
	if err != nil {
		t.Fatalf("ParseRefresh: %v", err)
	}
	return claims.ID
}
