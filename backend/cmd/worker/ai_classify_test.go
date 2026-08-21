package main

import (
	"context"
	"testing"
)

// TestAIClassifyDisabled 未配置 LLM 时静默（不产出 finding）。
func TestAIClassifyDisabled(t *testing.T) {
	fs := runAIClassify(context.Background(), "https://example.com/",
		[]byte("<html><body>普通内容</body></html>"))
	if len(fs) != 0 {
		t.Fatalf("未配置 LLM 应静默, got %d findings", len(fs))
	}
}

// TestAIConfigNotEnabled 未配置端点/Key 时 enabled 为 false。
func TestAIConfigNotEnabled(t *testing.T) {
	cfg := aiClassifyConfig{}
	if cfg.enabled() {
		t.Fatal("空配置不应启用")
	}
}

// TestAIClassifyPromptTooShort 文本过短不调用。
func TestAIClassifyPromptTooShort(t *testing.T) {
	cfg := aiClassifyConfig{BaseURL: "https://example.com/v1", APIKey: "x"}
	if cat, _ := aiClassify(context.Background(), cfg, "https://x.com/", "短"); cat != "" {
		t.Fatalf("短文本不应调用, got %s", cat)
	}
}

// TestAIClassifyNoModel 配置了 Key 但 endpoint 不可达 → 静默降级（不 panic）。
func TestAIClassifyNoModel(t *testing.T) {
	cfg := aiClassifyConfig{BaseURL: "https://127.0.0.1:1/v1", APIKey: "test-key", Model: "m"}
	if !cfg.enabled() {
		t.Fatal("配置完整应启用")
	}
	cat, conf := aiClassify(context.Background(), cfg, "https://x.com/", "这是一段足够长的测试文本内容用于分类判断。")
	if cat != "" || conf != 0 {
		t.Fatalf("不可达端点应降级为空, got %s/%.2f", cat, conf)
	}
}
