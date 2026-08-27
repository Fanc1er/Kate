package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeIntelProvider struct{ items []IntelItem }

func (f fakeIntelProvider) Query(ctx context.Context, target Target) ([]IntelItem, error) {
	return f.items, nil
}

func TestMatchCVE(t *testing.T) {
	f := MatchCVE("https://example.com", "CVE-2024-1234", "描述")
	if f.Type != "cve_match" {
		t.Fatalf("type = %s, want cve_match", f.Type)
	}
	if f.Severity != SeverityHigh {
		t.Fatalf("severity = %s, want high", f.Severity)
	}
	if f.URL != "https://example.com" {
		t.Fatalf("url = %s", f.URL)
	}
	cve, _ := f.Extra["cve_id"].(string)
	if cve != "CVE-2024-1234" {
		t.Fatalf("extra.cve_id = %s", cve)
	}
}

func TestExtractVersion(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"nginx/1.24.0", "1.24.0"},
		{"Apache/2.4.54 (Ubuntu)", "2.4.54"},
		{"plain", ""},
	}
	for _, c := range cases {
		if got := ExtractVersion(c.raw); got != c.want {
			t.Fatalf("ExtractVersion(%s) = %s, want %s", c.raw, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.22.0", "1.24.0", -1},
		{"1.24.0", "1.24.0", 0},
		{"1.26.0", "1.24.0", 1},
		{"1.9", "1.24", -1},
		{"2.4.54", "2.4.55", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Fatalf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestDetectComponent(t *testing.T) {
	cases := []struct {
		server string
		want   string
	}{
		{"nginx/1.22.0", "nginx"},
		{"Apache/2.4.54 (Ubuntu)", "apache"},
		{"Microsoft-IIS/10.0", "iis"},
		{"openresty/1.21.4", "openresty"},
		{"", ""},
	}
	for _, c := range cases {
		if got := detectComponent(c.server); got != c.want {
			t.Fatalf("detectComponent(%s) = %q, want %q", c.server, got, c.want)
		}
	}
}

func TestMatchIntelRules(t *testing.T) {
	if got := matchIntelRules("nginx", "1.22.0"); len(got) != 1 {
		t.Fatalf("nginx 1.22.0 应命中 1 条规则, got %d", len(got))
	}
	if got := matchIntelRules("nginx", "1.18.0"); len(got) != 2 {
		t.Fatalf("nginx 1.18.0 应命中 2 条规则, got %d", len(got))
	}
	if got := matchIntelRules("nginx", "1.26.0"); len(got) != 0 {
		t.Fatalf("nginx 1.26.0 不应命中, got %d", len(got))
	}
	if got := matchIntelRules("apache", "2.4.54"); len(got) != 1 {
		t.Fatalf("apache 2.4.54 应命中 1 条规则, got %d", len(got))
	}
	if got := matchIntelRules("unknown", "1.0.0"); len(got) != 0 {
		t.Fatalf("未知组件不应命中, got %d", len(got))
	}
}

func TestIntelligenceEngineBuiltinRule(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.22.0")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewIntelligenceEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var hasCVE bool
	for _, f := range fs {
		if f.Type == "cve_match" {
			hasCVE = true
			if cve, _ := f.Extra["cve_id"].(string); cve != "CVE-2023-44487" {
				t.Fatalf("cve_id = %s, want CVE-2023-44487", cve)
			}
		}
	}
	if !hasCVE {
		t.Fatalf("nginx 1.22.0 应命中内置规则, got %d", len(fs))
	}
}

func TestIntelligenceEngineNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.26.0")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewIntelligenceEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "cve_match" {
			t.Fatalf("nginx 1.26.0 不应命中 cve_match: %v", f)
		}
	}
}

func TestIntelligenceEngineProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	prov := fakeIntelProvider{items: []IntelItem{
		{ID: "CVE-2024-0001", Title: "测试情报", Description: "测试描述"},
	}}
	e := NewIntelligenceEngine().WithProvider(prov)
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	var hasProvider bool
	for _, f := range fs {
		if f.Type == "cve_match" {
			if cve, _ := f.Extra["cve_id"].(string); cve == "CVE-2024-0001" {
				hasProvider = true
			}
		}
	}
	if !hasProvider {
		t.Fatalf("外部情报源结果应产出 cve_match, got %d", len(fs))
	}
}

// HeaderIntelProvider 与内置规则同源时，同一 CVE 只应产出一次。
func TestIntelligenceEngineHeaderProviderDedup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.24.0")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := NewIntelligenceEngine().WithProvider(NewHeaderIntelProvider("nginx/1.24.0"))
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	counts := map[string]int{}
	for _, f := range fs {
		if f.Type == "cve_match" {
			cve, _ := f.Extra["cve_id"].(string)
			counts[cve]++
		}
	}
	for cve, n := range counts {
		if n > 1 {
			t.Fatalf("CVE %s 重复上报 %d 次", cve, n)
		}
	}
	if counts["CVE-2023-44487"] != 1 {
		t.Fatalf("nginx/1.24.0 应命中 CVE-2023-44487 恰好 1 次, got %d", counts["CVE-2023-44487"])
	}
}
