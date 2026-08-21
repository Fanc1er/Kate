package engines

import "testing"

func TestSensitiveInfoIDCard(t *testing.T) {
	e := NewSensitiveInfoEngine()
	fs, hits := e.Match("https://example.com", map[string]string{
		"response body": "联系人身份证: 110101199003071234",
	})
	if len(fs) != 1 || len(hits) == 0 {
		t.Fatalf("身份证应命中, fs=%d hits=%d", len(fs), len(hits))
	}
	if fs[0].Type != TypeSensitiveInfo {
		t.Fatalf("type = %s, want sensitive_info", fs[0].Type)
	}
	foundID := false
	for _, h := range hits {
		if h.Group == "身份证" {
			foundID = true
		}
	}
	if !foundID {
		t.Fatalf("应有身份证命中, got %+v", hits)
	}
}

func TestSensitiveInfoPhoneAndEmail(t *testing.T) {
	e := NewSensitiveInfoEngine()
	fs, hits := e.Match("https://example.com", map[string]string{
		"response body": "联系: 13800138000, email: user@example.com",
	})
	if len(fs) != 1 {
		t.Fatal("应产出主命中")
	}
	if len(hits) != 2 {
		t.Fatalf("应命中手机号+邮箱, got %d", len(hits))
	}
}

func TestSensitiveInfoJWTAndAK(t *testing.T) {
	e := NewSensitiveInfoEngine()
	fs, hits := e.Match("https://example.com", map[string]string{
		"response body": "token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U, ak: LTAI5tABCDEFGH1234",
	})
	if len(fs) != 1 {
		t.Fatal("应产出主命中")
	}
	groups := map[string]bool{}
	for _, h := range hits {
		groups[h.Group] = true
	}
	if !groups["JWT"] || !groups["云凭证"] {
		t.Fatalf("应命中 JWT+云凭证, got %+v", hits)
	}
}

func TestSensitiveInfoAuthorizationHeader(t *testing.T) {
	e := NewSensitiveInfoEngine()
	fs, hits := e.Match("https://example.com", map[string]string{
		"request header": "Authorization: Bearer abc.def.ghi",
	})
	if len(fs) != 1 || len(hits) == 0 {
		t.Fatalf("Authorization 头应命中, fs=%d hits=%d", len(fs), len(hits))
	}
	if hits[0].Scope != "request header" {
		t.Fatalf("scope = %s, want request header", hits[0].Scope)
	}
}

func TestSensitiveInfoCleanPage(t *testing.T) {
	e := NewSensitiveInfoEngine()
	fs, hits := e.Match("https://example.com", map[string]string{
		"response body": "欢迎访问我们的官方网站，提供优质服务。",
	})
	if len(fs) != 0 || len(hits) != 0 {
		t.Fatalf("干净页面不应命中, fs=%d hits=%d", len(fs), len(hits))
	}
}

func TestSensitiveInfoScopeMismatch(t *testing.T) {
	e := NewSensitiveInfoEngine()
	// 只提供 response body，但身份证规则 scope 也是 response body，验证 scope 隔离：Authorization 规则只查 header。
	fs, hits := e.Match("https://example.com", map[string]string{
		"response body": "Authorization: Bearer abc.def.ghi",
	})
	if len(fs) != 0 {
		t.Fatalf("body 中的 Authorization 不应命中 header 规则, got %+v", hits)
	}
}

func TestSensitiveInfoDisabled(t *testing.T) {
	e := NewSensitiveInfoEngine()
	if e.Enabled(Policy{Enabled: map[string]bool{NameSensitiveInfo: false}}) {
		t.Fatal("显式关闭后应禁用")
	}
	if !e.Enabled(Policy{Enabled: map[string]bool{NameSensitiveInfo: true, NameContentSecurity: false}}) {
		t.Fatal("子能力独立启用应覆盖总开关")
	}
}
