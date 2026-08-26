package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVulnScanEngineSensitivePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.env":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("DB_PASSWORD=secret"))
		case "/.git/config":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	e := NewVulnScanEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if len(fs) == 0 {
		t.Fatalf("应产出敏感路径暴露 finding, got 0")
	}
	for _, f := range fs {
		if f.Type != "path_exposure" {
			t.Fatalf("type = %s, want path_exposure", f.Type)
		}
		if f.Severity != SeverityMedium {
			t.Fatalf("severity = %s, want medium", f.Severity)
		}
	}
}

func TestVulnScanEngineNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	e := NewVulnScanEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if len(fs) != 0 {
		t.Fatalf("无敏感路径暴露不应产出 finding, got %d", len(fs))
	}
}

func TestVulnScanEngineContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := NewVulnScanEngine()
	_, err := e.Run(ctx, Target{URL: srv.URL}, Policy{})
	if err == nil {
		t.Fatalf("取消上下文应返回错误")
	}
}
