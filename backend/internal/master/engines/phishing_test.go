package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPhishingEngineNoHTTPS(t *testing.T) {
	e := NewPhishingEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "http://example.com"}, Policy{})
	var found bool
	for _, f := range fs {
		if f.Type == "no_https" {
			found = true
			if f.Severity != SeverityLow {
				t.Fatalf("severity = %s, want low", f.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("HTTP 目标应产出 no_https finding")
	}
}

func TestPhishingEngineHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	e := NewPhishingEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "no_https" {
			t.Fatalf("HTTPS 目标不应产出 no_https finding")
		}
	}
}

func TestIsSuspiciousDomain(t *testing.T) {
	cases := []struct {
		domain string
		want   bool
	}{
		{"1234567890.com", true},
		{"exa-mple-123-456.com", false},
		{"example.com", false},
	}
	for _, c := range cases {
		if got := isSuspiciousDomain(c.domain); got != c.want {
			t.Fatalf("isSuspiciousDomain(%s) = %v, want %v", c.domain, got, c.want)
		}
	}
}

func TestParseDomain(t *testing.T) {
	u, err := parseDomain("https://sub.example.com:8443/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u != "sub.example.com" {
		t.Fatalf("domain = %s, want sub.example.com", u)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"paypal.com", "paypa1.com", 1},
		{"google.com", "g00gle.com", 2},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Fatalf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestDetectTyposquatting(t *testing.T) {
	cases := []struct {
		domain string
		brand  string
	}{
		{"paypa1.com", "paypal.com"},
		{"g00gle.com", "google.com"},
		{"apple.com", ""},
		{"www.google.com", ""},
	}
	for _, c := range cases {
		brand, _ := detectTyposquatting(c.domain)
		if brand != c.brand {
			t.Fatalf("detectTyposquatting(%s) = %q, want %q", c.domain, brand, c.brand)
		}
	}
}

func TestPhishingEngineTyposquatting(t *testing.T) {
	e := NewPhishingEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "http://paypa1.com/login"}, Policy{})
	var hasTypos bool
	for _, f := range fs {
		if f.Type == "typosquatting" {
			hasTypos = true
			if f.Severity != SeverityHigh {
				t.Fatalf("severity = %s, want high", f.Severity)
			}
		}
	}
	if !hasTypos {
		t.Fatalf("仿冒域名应产出 typosquatting finding, got %d", len(fs))
	}
}

func TestPhishingEngineTLSCert(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	e := NewPhishingEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "cert_expired" || f.Type == "cert_hostname_mismatch" || f.Type == "cert_not_yet_valid" {
			t.Fatalf("本地有效证书不应产出 %s: %v", f.Type, f)
		}
	}
}

func TestCheckTLSCertificateClosed(t *testing.T) {
	// 连接失败应静默返回，不产出 finding。
	if fs := checkTLSCertificate("127.0.0.1:1", "127.0.0.1"); len(fs) != 0 {
		t.Fatalf("连接失败不应产出 finding, got %d", len(fs))
	}
}

func TestTLSServerHostnameMismatch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	_ = u
	// httptest 测试证书覆盖 127.0.0.1 / example.com / *.example.com，
	// 用不匹配的主机名校验应命中 mismatch。
	fs := checkTLSCertificate(u.Host, "not-matching.com")
	var hasMismatch bool
	for _, f := range fs {
		if f.Type == "cert_hostname_mismatch" {
			hasMismatch = true
		}
	}
	if !hasMismatch {
		t.Fatalf("不匹配主机名应产出 cert_hostname_mismatch, got %d", len(fs))
	}
}

