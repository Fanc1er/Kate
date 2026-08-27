package engines

import (
	"context"
	"testing"
)

func TestRuleReputationFetcherExactMatch(t *testing.T) {
	f := NewRuleReputationFetcher([]string{"evil.com", "bad.example.org"})
	score, err := f.Lookup(context.Background(), "evil.com")
	if err != nil || score != 95 {
		t.Fatalf("精确命中应返回 95, got %d err %v", score, err)
	}
	if score, _ := f.Lookup(context.Background(), "sub.evil.com"); score != 0 {
		t.Fatalf("精确规则不匹配子域, got %d", score)
	}
}

func TestRuleReputationFetcherSuffixMatch(t *testing.T) {
	f := NewRuleReputationFetcher([]string{"*.bad.io", ".worse.net"})
	if score, _ := f.Lookup(context.Background(), "a.bad.io"); score != 90 {
		t.Fatalf("*.bad.io 子域命中应返回 90, got %d", score)
	}
	if score, _ := f.Lookup(context.Background(), "bad.io"); score != 90 {
		t.Fatalf("裸根域命中应返回 90, got %d", score)
	}
	if score, _ := f.Lookup(context.Background(), "x.worse.net"); score != 90 {
		t.Fatalf(".worse.net 后缀命中应返回 90, got %d", score)
	}
	if score, _ := f.Lookup(context.Background(), "notbad.io"); score != 0 {
		t.Fatalf("前缀相似域不应命中, got %d", score)
	}
}

func TestRuleReputationFetcherEmpty(t *testing.T) {
	f := NewRuleReputationFetcher(nil)
	if score, _ := f.Lookup(context.Background(), "anything.com"); score != 0 {
		t.Fatalf("空规则库返回 0, got %d", score)
	}
	f2 := NewRuleReputationFetcher([]string{"", "   "})
	if score, _ := f2.Lookup(context.Background(), "anything.com"); score != 0 {
		t.Fatalf("空白规则返回 0, got %d", score)
	}
}

func TestHeaderIntelProviderServerMatch(t *testing.T) {
	p := NewHeaderIntelProvider("nginx/1.16.1")
	items, err := p.Query(context.Background(), Target{URL: "http://x.com"})
	if err != nil {
		t.Fatalf("Query err: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == "CVE-2021-23017" {
			found = true
		}
	}
	if !found {
		t.Fatalf("nginx/1.16.1 应命中 CVE-2021-23017, got %v", items)
	}
}

func TestHeaderIntelProviderPatchedVersionNoHit(t *testing.T) {
	p := NewHeaderIntelProvider("nginx/1.27.0")
	items, _ := p.Query(context.Background(), Target{})
	for _, it := range items {
		if it.ID == "CVE-2021-23017" {
			t.Fatalf("已修复版本不应命中 CVE-2021-23017")
		}
	}
}

func TestHeaderIntelProviderEmptyServer(t *testing.T) {
	p := NewHeaderIntelProvider("")
	items, _ := p.Query(context.Background(), Target{})
	if len(items) != 0 {
		t.Fatalf("无 Server 头不应产出情报条目, got %d", len(items))
	}
}

func TestHeaderIntelProviderExtraRules(t *testing.T) {
	p := NewHeaderIntelProvider("Tomcat/9.0.30").WithComponentRules([]ComponentRule{
		{
			Component: "tomcat", MaxVersion: "9.0.31", ID: "CVE-2020-9484",
			Title:       "Tomcat Session 持久化反序列化",
			Description: "文件上传路径配置不当可触发远程代码执行",
			Severity:    SeverityHigh,
		},
	})
	items, _ := p.Query(context.Background(), Target{})
	hit := false
	for _, it := range items {
		if it.ID == "CVE-2020-9484" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("扩展组件规则应命中 CVE-2020-9484, got %v", items)
	}
}
