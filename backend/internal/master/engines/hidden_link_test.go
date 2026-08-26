package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
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
