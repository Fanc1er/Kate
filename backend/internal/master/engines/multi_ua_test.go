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
		fmt.Fprint(w, "<html><head><title>ok</title></head><body><p>this is a normal page with enough content to avoid SPA shell detection</p></body></html>")
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
	if len(res.EndDown) != 1 || res.EndDown[0] != "all" {
		t.Fatalf("全端失败应标记 all, got %v", res.EndDown)
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

func TestMultiUADOMSimilarity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>t</title></head><body><div class='a'><p>a full paragraph with enough visible text for a real page body content</p></div></body></html>")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	// 同一页面各端 DOM 结构应高度相似。
	if res.DOMSimilarity < 90 {
		t.Fatalf("同页各端 DOM 相似度应 >90, got %d", res.DOMSimilarity)
	}
	if res.Score != 0 || res.Level != "正常" {
		t.Fatalf("一致页面应 0 分/正常, got %d/%s", res.Score, res.Level)
	}
}

func TestMultiUASPAShell(t *testing.T) {
	// SPA 空壳：几乎无可见文本但有 DOM 容器。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>SPA</title></head><body><div id="app"></div><script src="/app.js"></script></body></html>`)
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	if !res.SPASuspected {
		t.Fatal("空壳页面应标记 SPA 疑似")
	}
	// 特征分含 SPA +15。
	if res.FeatureScore < 15 {
		t.Fatalf("SPA 空壳特征分应 ≥15, got %d", res.FeatureScore)
	}
}

func TestMultiUATieredScoring(t *testing.T) {
	// 移动端定向失败 → 场景分含移动端加重。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("User-Agent"), "iPhone") {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "<html><body>ok</body></html>")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	// iPhone 相关探针（mobile/wechat）失败。
	if res.ScenarioScore < 30 {
		t.Fatalf("端差异化宕机场景分应 ≥30, got %d (down=%v)", res.ScenarioScore, res.EndDown)
	}
	// 移动端加重（+10）。
	if res.ScenarioScore < 40 {
		t.Fatalf("移动端定向投毒应加重场景分 ≥40, got %d", res.ScenarioScore)
	}
}

func TestMultiUADOMFingerprint(t *testing.T) {
	fp1 := domFingerprint("<html><body><div><p>a</p></div></body></html>")
	fp2 := domFingerprint("<html><body><div><p>a</p><span>b</span></div></body></html>")
	fp3 := domFingerprint("<html><body><div><p>a</p></div></body></html>")
	if fp1 == "" || fp2 == "" {
		t.Fatal("DOM 指纹不应为空")
	}
	if fp1 != fp3 {
		t.Fatal("相同结构指纹应一致")
	}
	if fp1 == fp2 {
		t.Fatal("不同结构指纹应不同")
	}
	// 相似结构汉明距离小。
	d := simHashDistance(fp1, fp2)
	if d < 0 || d > 20 {
		t.Fatalf("相似结构汉明距离应较小(0~20), got %d", d)
	}
}
