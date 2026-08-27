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
			_, _ = w.Write([]byte("DB_PASSWORD=secret\nAPP_KEY=abc123"))
		case "/.git/config":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[core]\n\trepositoryformatversion = 0"))
		case "/actuator/env":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
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
	if len(fs) != 3 {
		t.Fatalf("应产出 3 条 path_exposure, got %d", len(fs))
	}
	for _, f := range fs {
		if f.Type != "path_exposure" {
			t.Fatalf("type = %s, want path_exposure", f.Type)
		}
	}
	envConfirmed := false
	gitConfirmed := false
	actuatorPlain := false
	for _, f := range fs {
		switch path := f.Extra["path"]; path {
		case "/.env":
			envConfirmed = f.Extra["marker"] == "env_kv_pairs" && f.Severity == SeverityHigh && f.Confidence >= 0.85
		case "/.git/config":
			gitConfirmed = f.Extra["marker"] == "git_repo_file" && f.Severity == SeverityHigh
		case "/actuator/env":
			actuatorPlain = f.Severity == SeverityMedium && f.Extra["marker"] == nil
		}
	}
	if !envConfirmed || !gitConfirmed || !actuatorPlain {
		t.Fatalf("分级结果不符 env=%v git=%v actuator=%v", envConfirmed, gitConfirmed, actuatorPlain)
	}
}

func TestVulnScanEngineSoft404Suppressed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>welcome</body></html>"))
	}))
	defer srv.Close()

	e := NewVulnScanEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if len(fs) != 0 {
		t.Fatalf("软 404 站点无内容标记不应产出 finding, got %d", len(fs))
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

func TestIsSensitivePath(t *testing.T) {
	cases := map[string]bool{
		"/.env": true, "/.git/config": true, "/backup.sql": true,
		"/actuator/env": true, "/robots.txt": false, "/sitemap.xml": false,
	}
	for p, want := range cases {
		if got := isSensitivePath(p); got != want {
			t.Fatalf("isSensitivePath(%q) = %v, want %v", p, got, want)
		}
	}
}
