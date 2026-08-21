package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		base, raw, want string
	}{
		{"https://example.com/a", "/b", "https://example.com/b"},
		{"https://example.com/a/b", "../c", "https://example.com/c"},
		{"https://example.com/a#frag", "", "https://example.com/a"},
		{"https://example.com:443/a", "", "https://example.com/a"},
		{"http://example.com:80/a", "", "http://example.com/a"},
		{"https://example.com/a", "mailto:x@y.com", ""},
		{"https://example.com/a", "//cdn.example.com/lib.js", "https://cdn.example.com/lib.js"},
	}
	for _, c := range cases {
		got := normalizeURL(c.base, c.raw)
		if got != c.want {
			t.Errorf("normalizeURL(%q,%q)=%q want %q", c.base, c.raw, got, c.want)
		}
	}
}

func TestClassifyAsset(t *testing.T) {
	cases := []struct {
		page, asset, want string
	}{
		{"https://example.com/", "https://example.com/app.js", "js"},
		{"https://example.com/", "https://example.com/style.css", "css"},
		{"https://example.com/", "https://example.com/logo.png", "image"},
		{"https://example.com/", "https://example.com/hero.mp4", "video"},
		{"https://example.com/", "https://example.com/api/users/list", "api_path"},
		{"https://example.com/", "https://sub.example.com/", "subdomain"},
		{"https://example.com/", "https://example.com/", ""},
	}
	for _, c := range cases {
		got := classifyAsset(c.page, c.asset)
		if got != c.want {
			t.Errorf("classifyAsset(%q,%q)=%q want %q", c.page, c.asset, got, c.want)
		}
	}
}

func TestExtractAssets(t *testing.T) {
	html := `<html>
		<script src="/app.js"></script>
		<link rel="stylesheet" href="/style.css">
		<img src="/logo.png" srcset="/logo2x.png 2x, /thumb.png 1x">
		<video src="/demo.mp4"></video>
		<form action="/api/submit"></form>
		<a href="/page2">page2</a>
	</html>`
	assets := extractAssets("https://example.com/", html)
	got := map[string]string{}
	for _, a := range assets {
		got[a.URL] = a.SourceType
	}
	want := map[string]string{
		"https://example.com/app.js":    "js",
		"https://example.com/style.css": "css",
		"https://example.com/logo.png":  "image",
		"https://example.com/logo2x.png": "image",
		"https://example.com/thumb.png": "image",
		"https://example.com/demo.mp4":  "video",
		"https://example.com/api/submit": "api_path",
	}
	for u, st := range want {
		if got[u] != st {
			t.Errorf("asset %q: got %q want %q (all=%v)", u, got[u], st, got)
		}
	}
}

func TestRecursiveCrawlDepthAndDiscovery(t *testing.T) {
	var mux http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><body>
				<script src="/app.js"></script>
				<a href="/page2">p2</a>
			</body></html>`)
		case "/page2":
			fmt.Fprint(w, `<html><body>
				<img src="/img.png">
				<a href="/page3">p3</a>
			</body></html>`)
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body>leaf</body></html>")
		}
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	w := &worker{}
	p := &policyPayload{
		ScanDepth: 2, ConcurrencyLimit: 2, SameOrigin: true, CrawlSubpages: true,
		Engines: []string{"availability"},
	}
	assets, crawled, _, err := w.recursiveCrawl(context.Background(), srv.URL+"/", p, http.Header{})
	if err != nil {
		t.Fatalf("recursiveCrawl: %v", err)
	}
	// 深度 2：种子 + page2 被抓，page3 深度 3 超限。
	if crawled != 2 {
		t.Fatalf("应抓 2 个页面, got %d", crawled)
	}
	var foundJS, foundImg bool
	for _, a := range assets {
		if a.SourceType == "js" {
			foundJS = true
		}
		if a.SourceType == "image" {
			foundImg = true
		}
	}
	if !foundJS {
		t.Fatal("应发现 js 资产")
	}
	if !foundImg {
		t.Fatal("应发现 image 资产")
	}
}

func TestRecursiveCrawlDepthOneWhenNoSubpages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><a href="/sub">sub</a></html>`)
	}))
	defer srv.Close()

	w := &worker{}
	p := &policyPayload{
		ScanDepth: 3, ConcurrencyLimit: 2, SameOrigin: true, CrawlSubpages: false,
		Engines: []string{"availability"},
	}
	_, crawled, _, err := w.recursiveCrawl(context.Background(), srv.URL+"/", p, http.Header{})
	if err != nil {
		t.Fatalf("recursiveCrawl: %v", err)
	}
	// crawl_subpages=false 时深度强制 1，只抓种子页。
	if crawled != 1 {
		t.Fatalf("crawl_subpages=false 应只抓 1 页, got %d", crawled)
	}
}

func TestSameOriginFiltering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><a href="/internal">i</a><a href="https://evil.com/x">e</a></html>`)
	}))
	defer srv.Close()

	w := &worker{}
	p := &policyPayload{
		ScanDepth: 2, ConcurrencyLimit: 2, SameOrigin: true, CrawlSubpages: true,
		Engines: []string{"availability"},
	}
	_, _, _, err := w.recursiveCrawl(context.Background(), srv.URL+"/", p, http.Header{})
	if err != nil {
		t.Fatalf("recursiveCrawl: %v", err)
	}
	// 非孤儿：同域 /internal 被递归，evil.com 被过滤。这里验证不 panic 且无跨域抓取。
	// 直接验证 sameOriginOf 行为。
	if sameOriginOf(srv.URL+"/", srv.URL+"/internal") != true {
		t.Fatal("同域应判定 true")
	}
	if sameOriginOf(srv.URL+"/", "https://evil.com/x") {
		t.Fatal("跨域应判定 false")
	}
}

func TestStaticFiltering(t *testing.T) {
	// allow_static=false 时静态资源不递归抓取，但可作为资产发现。
	rc := &recursionConfig{AllowStatic: false}
	if !rc.isStaticFiltered("https://example.com/app.js") {
		t.Fatal("allow_static=false 时 .js 应被过滤")
	}
	rc2 := &recursionConfig{AllowStatic: true}
	if rc2.isStaticFiltered("https://example.com/app.js") {
		t.Fatal("allow_static=true 时 .js 不应被过滤")
	}
	if !strings.Contains(classifyAsset("https://example.com/", "https://example.com/x.js"), "js") {
		t.Fatal("js 资源分类错误")
	}
}
