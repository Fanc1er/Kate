package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHiddenLinkEngineHiddenElement(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<div style="display:none"><a href="http://evil.com">hidden</a></div>
			<a href="https://example.com/ok">ok</a>
		</body></html>`))
	}))
	defer srv.Close()

	e := NewHiddenLinkEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var hasHidden bool
	for _, f := range fs {
		if f.Type == "hidden_element" {
			hasHidden = true
		}
	}
	if !hasHidden {
		t.Fatalf("应产出 hidden_element finding, got %d findings", len(fs))
	}
}

func TestHiddenLinkEngineExternalIframe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<iframe src="https://evil.example.com/banner"></iframe>
		</body></html>`))
	}))
	defer srv.Close()

	e := NewHiddenLinkEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	var hasIframe bool
	for _, f := range fs {
		if f.Type == "external_iframe" {
			hasIframe = true
		}
	}
	if !hasIframe {
		t.Fatalf("应产出 external_iframe finding, got %d findings", len(fs))
	}
}

func TestHiddenLinkIframeDedup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<iframe src="https://evil.example.com/ad"></iframe>
			<iframe src="https://evil.example.com/ad"></iframe>
		</body></html>`)
	}))
	defer srv.Close()

	fs, _ := NewHiddenLinkEngine().Run(context.Background(), Target{URL: srv.URL}, Policy{})
	count := 0
	for _, f := range fs {
		if f.Type == "external_iframe" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("重复 src iframe 应去重为 1 条, got %d", count)
	}
}

func TestHiddenLinkSameSiteIFrameSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/embed":
			_, _ = w.Write([]byte("<html>inner</html>"))
		default:
			fmt.Fprintf(w, `<html><body><iframe src="/embed"></iframe></body></html>`)
		}
	}))
	defer srv.Close()

	fs, _ := NewHiddenLinkEngine().Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "external_iframe" {
			t.Fatalf("同站 iframe 不应上报, got %+v", f)
		}
	}
}

func TestHiddenLinkLineNumbers(t *testing.T) {
	lines := []string{
		"<html><body>",
		"<p>spacer</p>",
		"<p>spacer</p>",
		`<iframe src="https://evil.example.com/x"></iframe>`,
		"</body></html>",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Join(lines, "\n"))
	}))
	defer srv.Close()

	fs, _ := NewHiddenLinkEngine().Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "external_iframe" {
			if f.LineNo != 4 {
				t.Fatalf("external_iframe LineNo = %d, want 4", f.LineNo)
			}
			return
		}
	}
	t.Fatalf("未产出 external_iframe")
}

func TestHiddenLinkAnchorJSProtocolScoped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<p>关于 javascript: 的说明文字，属于普通文案不应误报</p>
			<a href="javascript:void(0)">点我</a>
			<a href="/data/report.png">报表</a>
		</body></html>`))
	}))
	defer srv.Close()

	fs, _ := NewHiddenLinkEngine().Run(context.Background(), Target{URL: srv.URL}, Policy{})
	for _, f := range fs {
		if f.Type == "javascript_protocol" {
			if f.LineNo != 3 {
				t.Fatalf("javascript_protocol LineNo = %d, want 3", f.LineNo)
			}
			return
		}
		if f.Type == "data_uri" {
			t.Fatalf("普通 data 路径文件不应命中 data_uri")
		}
	}
	t.Fatalf("锚点 javascript: 协议应被检出")
}

func TestHiddenLinkDataURIAnchorDetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<img src="data:image/png;base64,iVBOR">
			<a href="data:text/html,<script>alert(1)</script>">预览</a>
		</body></html>`))
	}))
	defer srv.Close()

	fs, _ := NewHiddenLinkEngine().Run(context.Background(), Target{URL: srv.URL}, Policy{})
	found := false
	for _, f := range fs {
		if f.Type == "data_uri" && strings.Contains(f.Description, "text/html") {
			found = true
		}
	}
	if !found {
		t.Fatalf("可执行 data:text/html 锚点应被检出")
	}
}

func TestHiddenLinkEngineClean(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
			<a href="/about">关于我们</a>
		</body></html>`))
	}))
	defer srv.Close()

	e := NewHiddenLinkEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if len(fs) != 0 {
		t.Fatalf("干净页面不应产出 finding, got %d", len(fs))
	}
}

func TestExtractLinks(t *testing.T) {
	html := `<a href="/a">A</a><a href="https://b.com">B</a><a href="#frag">C</a>`
	links := extractLinks(html)
	if len(links) != 3 {
		t.Fatalf("extractLinks = %d, want 3", len(links))
	}
}
