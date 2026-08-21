package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeadLinkEngineHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewDeadLinkEngine()
	fs := e.Check(context.Background(), srv.URL, []string{srv.URL + "/ok"}, Policy{})
	if len(fs) != 0 {
		t.Fatalf("健康链接不应视为死链, got %v", fs)
	}
}

func TestDeadLinkEngine404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := NewDeadLinkEngine()
	fs := e.Check(context.Background(), srv.URL, []string{srv.URL + "/missing"}, Policy{})
	if len(fs) != 1 {
		t.Fatalf("404 应产出死链, got %d", len(fs))
	}
	if fs[0].Type != TypeDeadLink {
		t.Fatalf("type = %s, want dead_link", fs[0].Type)
	}
	st, _ := fs[0].Extra["status"].(int)
	if st != 404 {
		t.Fatalf("status = %v, want 404", fs[0].Extra["status"])
	}
}

func TestDeadLinkEngine500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := NewDeadLinkEngine()
	fs := e.Check(context.Background(), srv.URL, []string{srv.URL + "/error"}, Policy{})
	if len(fs) != 1 {
		t.Fatalf("500 应产出死链, got %d", len(fs))
	}
	if fs[0].Severity != SeverityHigh {
		t.Fatalf("severity = %s, want high", fs[0].Severity)
	}
}

func TestDeadLinkEngineConnectionRefused(t *testing.T) {
	e := NewDeadLinkEngine()
	// 本地未监听端口 → 连接失败。
	fs := e.Check(context.Background(), "http://127.0.0.1:1", []string{"http://127.0.0.1:1/x"}, Policy{})
	if len(fs) != 1 {
		t.Fatalf("连接失败应产出死链, got %d", len(fs))
	}
	st, _ := fs[0].Extra["status"].(string)
	if st != "conn_error" {
		t.Fatalf("status = %v, want conn_error", fs[0].Extra["status"])
	}
}

func TestDeadLinkEngineResolveRelative(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := NewDeadLinkEngine()
	// 相对链接基于 base 解析。
	fs := e.Check(context.Background(), srv.URL+"/page", []string{"/missing"}, Policy{})
	if len(fs) != 1 {
		t.Fatalf("相对链接应解析并产出死链, got %d", len(fs))
	}
	if !strings.Contains(fs[0].URL, "/missing") {
		t.Fatalf("URL 未归一化: %s", fs[0].URL)
	}
}

func TestDeadLinkEngineDedupe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := NewDeadLinkEngine()
	// 同一链接重复出现只产出一条。
	fs := e.Check(context.Background(), srv.URL, []string{srv.URL + "/a", srv.URL + "/a"}, Policy{})
	if len(fs) != 1 {
		t.Fatalf("重复链接应去重, got %d", len(fs))
	}
}
