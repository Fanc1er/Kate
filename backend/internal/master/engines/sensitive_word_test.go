package engines

import "testing"

func TestSensitiveWordHit(t *testing.T) {
	e := NewSensitiveWordEngine()
	fs := e.Match("https://example.com", "首页", "本站提供时时彩投注与赌博服务", nil)
	if len(fs) == 0 {
		t.Fatal("应命中敏感词")
	}
	if fs[0].Type != TypeSensitiveWord {
		t.Fatalf("type = %s, want sensitive_word", fs[0].Type)
	}
	if fs[0].Severity != SeverityHigh {
		t.Fatalf("severity = %s, want high", fs[0].Severity)
	}
	src, _ := fs[0].Extra["source"].(string)
	if src != "regex" {
		t.Fatalf("source = %s, want regex", src)
	}
}

func TestSensitiveWordCleanPage(t *testing.T) {
	e := NewSensitiveWordEngine()
	fs := e.Match("https://example.com", "首页", "欢迎访问我们的官方网站，提供优质服务。", nil)
	if len(fs) != 0 {
		t.Fatalf("干净页面不应命中, got %v", fs)
	}
}

func TestSensitiveWordWhitelistExclude(t *testing.T) {
	e := NewSensitiveWordEngine()
	// 白名单含命中词，应剔除。
	fs := e.Match("https://example.com", "赌博", "彩票内幕揭露文档",
		[]string{"彩票内幕"})
	if len(fs) != 0 {
		t.Fatalf("白名单剔除后不应命中, got %v", fs)
	}
	// 正则白名单。
	fs2 := e.Match("https://example.com", "博彩", "博彩行业研究报告",
		[]string{"regex:博彩行业"})
	if len(fs2) != 0 {
		t.Fatalf("正则白名单剔除后不应命中, got %v", fs2)
	}
}

func TestSensitiveWordDisabled(t *testing.T) {
	e := NewSensitiveWordEngine()
	if e.Enabled(Policy{Enabled: map[string]bool{NameContentSecurity: false}}) {
		t.Fatal("显式关闭后应禁用")
	}
	if !e.Enabled(Policy{Enabled: map[string]bool{NameContentSecurity: true}}) {
		t.Fatal("启用后应允许")
	}
	// 子能力独立开关。
	if e.Enabled(Policy{Enabled: map[string]bool{NameSensitiveWord: false}}) {
		t.Fatal("子能力关闭后应禁用")
	}
	if !e.Enabled(Policy{Enabled: map[string]bool{NameSensitiveWord: true, NameContentSecurity: false}}) {
		t.Fatal("子能力独立启用应覆盖总开关")
	}
}

func TestSensitiveWordAddCustomRule(t *testing.T) {
	e := NewSensitiveWordEngine()
	if err := e.AddBuiltin("自定义违规", "custom", `内购返利`); err != nil {
		t.Fatalf("AddBuiltin: %v", err)
	}
	fs := e.Match("https://example.com", "", "平台提供内购返利活动", nil)
	if len(fs) == 0 {
		t.Fatal("自定义规则应命中")
	}
}

func TestSensitiveWordTitleHit(t *testing.T) {
	e := NewSensitiveWordEngine()
	// 仅标题含敏感词也应命中。
	fs := e.Match("https://example.com", "冰毒交易", "普通正文内容", nil)
	if len(fs) == 0 {
		t.Fatal("标题命中应产出 finding")
	}
}
