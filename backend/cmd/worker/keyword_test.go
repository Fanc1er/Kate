package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/engines"
)

func TestKeywordMatchContent(t *testing.T) {
	p := &policyPayload{
		Engines: []string{"keyword"},
		KeywordRules: []keywordRule{
			{ID: 1, Name: "竞品词", Pattern: `超级竞品`},
			{ID: 2, Name: "敏感词", Pattern: `违规内容`, Sensitive: true},
			{ID: 3, Name: "带过滤规则", Pattern: `活动`, SRegex: `公司活动`},
		},
	}
	fs := runContentEngines(context.Background(), "https://example.com/page",
		[]byte("<html><body>欢迎参加超级竞品推广 违规内容不可出现</body></html>"),
		p, http.Header{})

	types := map[string]int{}
	for _, f := range fs {
		types[f.Type]++
	}
	if types["keyword_hit"] == 0 {
		t.Fatalf("应产出 keyword_hit，实际: %v", types)
	}
	// 敏感级规则应为 high。
	foundHigh := false
	for _, f := range fs {
		if f.Type == "keyword_hit" && f.Severity == engines.SeverityHigh {
			foundHigh = true
		}
		if f.Type == "keyword_hit" && f.Severity != engines.SeverityHigh && f.Severity != engines.SeverityMedium {
			t.Fatalf("keyword_hit 严重级别非法: %s", f.Severity)
		}
	}
	if !foundHigh {
		t.Fatal("敏感级规则应产出 high 告警")
	}
}

func TestKeywordMatchDisabledEngine(t *testing.T) {
	p := &policyPayload{Engines: []string{"availability"}}
	fs := runContentEngines(context.Background(), "https://example.com/",
		[]byte("<html>超级竞品</html>"), p, http.Header{})
	for _, f := range fs {
		if f.Type == "keyword_hit" {
			t.Fatal("keyword 引擎关闭时不应产出 keyword_hit")
		}
	}
}

func TestKeywordMatchURL(t *testing.T) {
	p := &policyPayload{
		Engines: []string{"keyword"},
		KeywordRules: []keywordRule{
			{ID: 1, Name: "URL 规则", Pattern: `/admin/`},
		},
	}
	// 正文不含 /admin/，仅 URL 命中。
	fs := runContentEngines(context.Background(), "https://example.com/admin/config",
		[]byte("<html><body>普通内容</body></html>"), p, http.Header{})
	hit := false
	for _, f := range fs {
		if f.Type == "keyword_hit" {
			hit = true
			if f.URL != "https://example.com/admin/config" {
				t.Fatalf("URL 错误: %s", f.URL)
			}
		}
	}
	if !hit {
		t.Fatal("URL 应命中 keyword 规则")
	}
}

func TestKeywordRuleInvalidRegex(t *testing.T) {
	p := &policyPayload{
		Engines: []string{"keyword"},
		KeywordRules: []keywordRule{
			{ID: 1, Name: "非法正则", Pattern: `[`},
			{ID: 2, Name: "正常规则", Pattern: `正常`},
		},
	}
	fs := runContentEngines(context.Background(), "https://example.com/",
		[]byte("<html>正常内容</html>"), p, http.Header{})
	// 非法正则规则被跳过，正常规则仍命中。
	hit := false
	for _, f := range fs {
		if f.Type == "keyword_hit" && f.Severity == engines.SeverityMedium {
			hit = true
		}
	}
	if !hit {
		t.Fatal("非法正则被跳过，正常规则应命中")
	}
}
