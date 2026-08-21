package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/engines"
)

// AI 文本分类：调用用户配置的 LLM 对页面正文做涉黄赌毒政分类。
// LLM 端点/Key 由 CINSIGHT_LLM_* 环境变量提供（用户自行配置），未配置或超时回退静默
// （不产出 AI finding，敏感词正则双判定由 sensitive_word 覆盖）。

// aiClassifyConfig 从环境读取 LLM 配置。
type aiClassifyConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func loadAIConfig() aiClassifyConfig {
	return aiClassifyConfig{
		BaseURL: strings.TrimRight(os.Getenv("CINSIGHT_LLM_BASE_URL"), "/"),
		APIKey:  os.Getenv("CINSIGHT_LLM_API_KEY"),
		Model:   os.Getenv("CINSIGHT_LLM_MODEL"),
	}
}

// aiClassifyEnabled 是否配置了 LLM。
func (c aiClassifyConfig) enabled() bool {
	return c.BaseURL != "" && c.APIKey != ""
}

// aiClassifyResponse LLM 分类响应（OpenAI 兼容格式）。
type aiClassifyResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// aiClassify 对正文做 AI 分类，返回命中分类名（空=未命中/不可用）。
func aiClassify(ctx context.Context, cfg aiClassifyConfig, pageURL, text string) (string, float64) {
	if !cfg.enabled() || len(text) < 10 {
		return "", 0
	}
	prompt := fmt.Sprintf(
		"对以下网页正文进行分类，判断是否包含涉黄、涉赌、涉毒、政治敏感等违规内容。仅回答一个分类词（黄/赌/毒/政/正常），并给出置信度0-1。正文：%s",
		truncate(text, 800),
	)
	payload, _ := json.Marshal(map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1, "max_tokens": 32,
	})
	url := cfg.BaseURL
	if !strings.Contains(url, "/chat/completions") {
		url += "/chat/completions"
	}
	ctx2, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx2, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", 0
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed aiClassifyResponse
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", 0
	}
	content := strings.ToLower(strings.TrimSpace(parsed.Choices[0].Message.Content))
	for _, cat := range []string{"黄", "赌", "毒", "政"} {
		if strings.Contains(content, cat) {
			conf := 0.8
			return cat, conf
		}
	}
	return "", 0
}

// runAIClassify 对页面文本执行 AI 分类，命中违规分类产出 content_violation finding。
// 未配置 LLM 或超时静默返回空。
func runAIClassify(ctx context.Context, pageURL string, body []byte) []findingPayload {
	cfg := loadAIConfig()
	if !cfg.enabled() {
		return nil
	}
	_, text := extractHTMLText(string(body))
	if strings.TrimSpace(text) == "" {
		return nil
	}
	cat, conf := aiClassify(ctx, cfg, pageURL, text)
	if cat == "" {
		return nil
	}
	label := map[string]string{"黄": "涉黄", "赌": "涉赌", "毒": "涉毒", "政": "政治敏感"}[cat]
	return []findingPayload{{
		EngineName: "content_security", Type: "content_violation", Severity: engines.SeverityHigh,
		Title:       "AI 文本分类: " + label,
		Description: fmt.Sprintf("AI 文本分类判定页面含%s内容（置信度 %.2f），来源: ai", label, conf),
		URL:         pageURL, Confidence: conf,
		Extra:       map[string]any{"category": cat, "source": "ai", "confidence": conf},
	}}
}
