package main

import (
	"context"
	"net/http"
	"testing"
)

func TestExtractExternalLinks(t *testing.T) {
	html := `<html>
		<a href="https://partner.com/page">外部链接</a>
		<a href="/internal-page">内部链接</a>
		<a href="#anchor">锚点</a>
		<a href="javascript:void(0)">脚本</a>
		<script src="https://cdn.example.net/lib.js"></script>
		<link rel="stylesheet" href="https://fonts.example.org/font.css">
		<img src="https://img.example.org/logo.png">
	</html>`
	links := extractExternalLinks("https://example.com/home", html)

	found := map[string]string{}
	for _, l := range links {
		found[l.URL] = l.Type
	}
	// 外部出站链接归 third_party_domain（不同根域）。
	if _, ok := found["https://partner.com/page"]; !ok {
		t.Fatalf("应检出外部链接 partner.com, got %v", found)
	}
	if typ := found["https://partner.com/page"]; typ != "third_party_domain" {
		t.Fatalf("外部链接类型应为 third_party_domain, got %s", typ)
	}
	// 内部链接 /internal-page 不应出现（同源）。
	if _, ok := found["/internal-page"]; ok {
		t.Fatal("内部链接不应视为外链")
	}
	// 外部资源。
	if _, ok := found["https://cdn.example.net/lib.js"]; !ok {
		t.Fatalf("应检出外部 js, got %v", found)
	}
	if _, ok := found["https://fonts.example.org/font.css"]; !ok {
		t.Fatal("应检出外部 css")
	}
	if _, ok := found["https://img.example.org/logo.png"]; !ok {
		t.Fatal("应检出外部图片")
	}
	// 锚点/脚本不检出。
	for u := range found {
		if u == "#anchor" || u == "javascript:void(0)" {
			t.Fatalf("不应检出 %s", u)
		}
	}
}

func TestEvaluateExternalLinks(t *testing.T) {
	rules := []domainRule{
		{Kind: "domain_whitelist", Pattern: "trusted.com"},
		{Kind: "malicious_domain", Pattern: "evil.com"},
	}
	links := []externalLink{
		{URL: "https://evil.com/x", Domain: "evil.com"},
		{URL: "https://trusted.com/y", Domain: "trusted.com"},
		{URL: "https://gooogle.com/z", Domain: "gooogle.com"}, // 仿冒 gooogle
	}
	res := evaluateExternalLinks(links, "google.com", rules)
	byURL := map[string]externalLink{}
	for _, l := range res {
		byURL[l.URL] = l
	}
	if !byURL["https://evil.com/x"].Suspicious {
		t.Fatal("恶意域名库命中应标记可疑")
	}
	if byURL["https://trusted.com/y"].Suspicious {
		t.Fatal("白名单域名不应标记可疑")
	}
	if !byURL["https://gooogle.com/z"].Suspicious {
		t.Fatal("域名相似度（gooogle.com vs google.com）应标记可疑")
	}
}

func TestDomainSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want bool // >=0.85
	}{
		{"google.com", "gooogle.com", true},
		{"google.com", "google.com", true}, // 相同 = 1
		{"google.com", "amazon.com", false},
		{"google.com", "evil.com", false},
	}
	for _, c := range cases {
		if got := domainSimilarity(c.a, c.b) >= 0.85; got != c.want {
			t.Fatalf("domainSimilarity(%s,%s) >= 0.85 = %v, want %v (val=%f)", c.a, c.b, got, c.want, domainSimilarity(c.a, c.b))
		}
	}
}

func TestExternalLinkFinding(t *testing.T) {
	p := &policyPayload{
		Engines: []string{"external_link"},
		DomainRules: []domainRule{
			{Kind: "malicious_domain", Pattern: "evil.com"},
		},
	}
	html := `<html><body><a href="https://evil.com/hack">bad</a><a href="https://example.com/ok">self</a></body></html>`
	fs := runContentEngines(context.Background(), "https://example.com/", []byte(html), p, http.Header{})

	var f *findingPayload
	for i := range fs {
		if fs[i].Type == "external_link" {
			f = &fs[i]
			break
		}
	}
	if f == nil {
		t.Fatal("应产出 external_link finding")
	}
	links, ok := f.Extra["external_links"].([]externalLink)
	if !ok {
		t.Fatalf("external_links 类型应为 []externalLink, got %T", f.Extra["external_links"])
	}
	if len(links) != 1 {
		t.Fatalf("应检出 1 条外链（evil.com，example.com 同源排除）, got %d: %+v", len(links), links)
	}
	if !links[0].Suspicious {
		t.Fatal("恶意域名 evil.com 应标记可疑")
	}
	if sc, ok := f.Extra["suspicious_count"].(int); !ok || sc != 1 {
		t.Fatalf("suspicious_count 应为 1, got %v", f.Extra["suspicious_count"])
	}
}
