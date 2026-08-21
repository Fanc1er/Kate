package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMultiUAAllProbesOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>ok</title></head><body>hello</body></html>")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	if len(res.Probes) != 4 {
		t.Fatalf("探针数 = %d, want 4", len(res.Probes))
	}
	for _, pr := range res.Probes {
		if pr.Failed || pr.StatusCode != http.StatusOK {
			t.Fatalf("探针 %s 异常: %+v", pr.Name, pr)
		}
	}
	if len(res.EndDown) != 0 || len(res.EndDiff) != 0 {
		t.Fatalf("全端一致不应有端级异常: down=%v diff=%v", res.EndDown, res.EndDiff)
	}
	if res.Score != 0 || res.Level != "正常" {
		t.Fatalf("score=%d level=%s, want 0/正常", res.Score, res.Level)
	}
}

func TestMultiUAProbeFailures(t *testing.T) {
	// 一个探针失败（目标端口未监听），其余正常。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	base := srv.URL

	e := NewMultiUAAssessor()
	// 用 dead 地址模拟失败探针：直接跑一次端口 1（未监听）。
	dead := "http://127.0.0.1:1/"
	res := e.Assess(context.Background(), dead, 3*1000*1000*1000)
	for _, pr := range res.Probes {
		if !pr.Failed {
			t.Fatalf("未监听端口应全部失败, %s: %+v", pr.Name, pr)
		}
	}
	if len(res.EndDown) != 4 {
		t.Fatalf("全端失败应标记 4 个宕机端, got %v", res.EndDown)
	}
	_ = base
}

func TestMultiUADisabled(t *testing.T) {
	e := NewMultiUAAssessor()
	if e.Enabled(Policy{Enabled: map[string]bool{NameMultiUA: false}}) {
		t.Fatal("显式关闭后应禁用")
	}
	if !e.Enabled(Policy{Enabled: map[string]bool{NameMultiUA: true}}) {
		t.Fatal("启用后应允许")
	}
}

func TestMultiUAEndDownFinding(t *testing.T) {
	// 构造 PC 正常 + 移动端 404 的端差异化场景。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("User-Agent"), "iPhone") || strings.Contains(r.Header.Get("User-Agent"), "Mobile") {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("端差异化宕机应产出 finding")
	}
	if fs[0].Type != TypeMultiUAAvailability {
		t.Fatalf("type = %s, want multi_ua_availability", fs[0].Type)
	}
	endDown, _ := fs[0].Extra["end_down"].([]string)
	if len(endDown) == 0 {
		t.Fatalf("应标记宕机端, got %v", fs[0].Extra)
	}
}
