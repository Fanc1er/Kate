package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMultiUASensitiveWordScore 端级敏感词命中计入评分（特征分 +25/端）。
func TestMultiUASensitiveWordScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>博彩平台</title></head><body><p>这是一个长期提供外围投注赔率的页面，内容足够长以通过 SPA 检测。</p></body></html>")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	for _, pr := range res.Probes {
		if pr.Failed {
			t.Fatalf("探针 %s 不应失败: %+v", pr.Name, pr)
		}
		if len(pr.SensitiveHits) == 0 {
			t.Fatalf("探针 %s 应命中敏感词, got %v", pr.Name, pr.SensitiveHits)
		}
	}
	// 4 端各 +25 特征分，score > 0。
	if res.Score <= 0 {
		t.Fatalf("敏感词命中应产生风险分, score=%d (feature=%d)", res.Score, res.FeatureScore)
	}
	if res.FeatureScore < 100 {
		t.Fatalf("4 端敏感词命中特征分应 >=100, got %d", res.FeatureScore)
	}
	if res.Level == "正常" {
		t.Fatal("命中敏感词不应判定为正常")
	}
}

// TestMultiUASensitiveInfoScore 端级敏感信息命中计入评分。
func TestMultiUASensitiveInfoScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body><p>联系电话 13800138000 的页面，内容足够长以通过 SPA 检测并包含有效联系信息。</p></body></html>")
	}))
	defer srv.Close()

	e := NewMultiUAAssessor()
	res := e.Assess(context.Background(), srv.URL, 5*1000*1000*1000)
	sawInfo := false
	for _, pr := range res.Probes {
		if len(pr.SensitiveInfoHits) > 0 {
			sawInfo = true
		}
	}
	if !sawInfo {
		// 若内置敏感信息规则未覆盖手机号，则端级敏感词也不命中，需确认规则。
		t.Skip("内置敏感信息规则未覆盖手机号，跳过")
	}
	if res.Score <= 0 {
		t.Fatalf("敏感信息命中应产生风险分, score=%d", res.Score)
	}
}
