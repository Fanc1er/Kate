package engines

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const NameHiddenLink = "hidden_link"

var (
	reDisplayNone = regexp.MustCompile(`(?i)display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0|text-indent\s*:\s*-\d+px|position\s*:\s*absolute.*?left\s*:\s*-\d+px`)
	reIFrame      = regexp.MustCompile(`(?i)<iframe[^>]*src\s*=\s*["']([^"']+)["']`)
	reHiddenLink  = regexp.MustCompile(`(?i)<a[^>]*href\s*=\s*["']([^"']+)["'][^>]*>.*?</a>`)
	reJavaScript  = regexp.MustCompile(`(?i)javascript\s*:`)
	reDataURI     = regexp.MustCompile(`(?i)data\s*:`)
)

// HiddenLinkEngine 暗链挂马引擎骨架：检测隐藏链接、可疑 iframe、javascript: 协议等。
type HiddenLinkEngine struct {
	client *http.Client
}

// NewHiddenLinkEngine 构造暗链引擎。
func NewHiddenLinkEngine() *HiddenLinkEngine {
	return &HiddenLinkEngine{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Name 返回引擎名。
func (e *HiddenLinkEngine) Name() string { return NameHiddenLink }

// Enabled 策略开关：hidden_link 未显式关闭即启用。
func (e *HiddenLinkEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameHiddenLink]
	if !ok {
		return true
	}
	return en
}

// Run 执行暗链检测。
func (e *HiddenLinkEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	resp, err := e.client.Get(target.URL)
	if err != nil {
		return findings, err
	}
	defer resp.Body.Close()
	buf := make([]byte, 1<<20) // 1MB limit
	n, _ := resp.Body.Read(buf)
	html := string(buf[:n])

	// 检测隐藏块元素
	if reDisplayNone.MatchString(html) {
		findings = append(findings, Finding{
			Type:        "hidden_element",
			Severity:    SeverityMedium,
			Title:       "检测到隐藏页面元素",
			Description: "页面包含 display:none / visibility:hidden 等隐藏样式，可能存在暗链或恶意内容",
			URL:         target.URL,
			Confidence:  0.5,
			Extra: map[string]any{
				"detection": "css_hidden",
			},
		})
	}

	// 检测 iframe 嵌入
	matches := reIFrame.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			src := m[1]
			if strings.HasPrefix(src, "http") && !strings.Contains(src, target.URL) {
				findings = append(findings, Finding{
					Type:        "external_iframe",
					Severity:    SeverityHigh,
					Title:       fmt.Sprintf("外部 iframe 嵌入：%s", src),
					Description: fmt.Sprintf("页面嵌入了外部 iframe: %s", src),
					URL:         target.URL,
					Confidence:  0.7,
					LineNo:      0,
					Extra: map[string]any{
						"iframe_src": src,
					},
				})
			}
		}
	}

	// 检测 javascript: 协议
	if reJavaScript.MatchString(html) {
		findings = append(findings, Finding{
			Type:        "javascript_protocol",
			Severity:    SeverityMedium,
			Title:       "检测到 javascript: 协议链接",
			Description: "页面包含 javascript: 协议链接，可能存在 XSS 或恶意跳转风险",
			URL:         target.URL,
			Confidence:  0.6,
		})
	}

	// 检测 data: URI
	if reDataURI.MatchString(html) {
		findings = append(findings, Finding{
			Type:        "data_uri",
			Severity:    SeverityLow,
			Title:       "检测到 data: URI 引用",
			Description: "页面包含 data: URI，需人工复核是否存在恶意内容",
			URL:         target.URL,
			Confidence:  0.4,
		})
	}

	return findings, nil
}

// extractLinks 从 HTML 中提取所有 href 链接。
func extractLinks(html string) []string {
	var links []string
	matches := reHiddenLink.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 {
			links = append(links, m[1])
		}
	}
	return links
}

// resolveURL 解析相对链接为绝对 URL（定义在 dead_link.go）。
