package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const NameHiddenLink = "hidden_link"

var (
	reDisplayNone = regexp.MustCompile(`(?i)display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0|text-indent\s*:\s*-\d+px|position\s*:\s*absolute[^"']*?left\s*:\s*-\d+px`)
	reIFrameSrc   = regexp.MustCompile(`(?i)<iframe\b[^>]*?\bsrc\s*=\s*["']([^"']+)["']`)
	reAnchorHref  = regexp.MustCompile(`(?i)<a\b[^>]*?\bhref\s*=\s*["']([^"']*)["']`)
	reJSProto     = regexp.MustCompile(`(?i)^\s*(javascript|vbscript)\s*:`)
	reDataURI     = regexp.MustCompile(`(?i)^\s*data\s*:\s*(text/html|application/xhtml|image/svg)`)
	reHiddenLink  = regexp.MustCompile(`(?i)<a[^>]*href\s*=\s*["']([^"']+)["'][^>]*>.*?</a>`)
)

// maxProtocolSamples 协议类链接聚合上报时展示的样例数上限。
const maxProtocolSamples = 5

// HiddenLinkEngine 暗链挂马引擎：检测隐藏元素、外部 iframe、危险协议链接。
type HiddenLinkEngine struct {
	client *http.Client
}

// NewHiddenLinkEngine 构造暗链引擎。
func NewHiddenLinkEngine() *HiddenLinkEngine {
	return &HiddenLinkEngine{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// WithClient 注入自定义 HTTP 客户端。
func (e *HiddenLinkEngine) WithClient(c *http.Client) *HiddenLinkEngine {
	if c != nil {
		e.client = c
	}
	return e
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

// Run 抓取目标页面并执行暗链检测：
// - CSS 隐藏块（display:none 等）配合锚点可能藏链；
// - 跨域 iframe 判定外部嵌入，重复 src 去重；
// - javascript:/vbscript:/危险 data: 协议按属性行定位，聚合上报。
func (e *HiddenLinkEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CInsight-Scanner/0.1")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	html := string(body)
	var findings []Finding

	// 隐藏样式块。
	if reDisplayNone.MatchString(html) {
		findings = append(findings, Finding{
			Type:        "hidden_element",
			Severity:    SeverityMedium,
			Title:       "检测到隐藏页面元素",
			Description: "页面包含 display:none / visibility:hidden 等隐藏样式，可能存在暗链或恶意内容",
			URL:         target.URL,
			Confidence:  0.5,
			Extra:       map[string]any{"detection": "css_hidden"},
		})
	}

	// 外部 iframe：相对地址先解析为绝对地址，同站内嵌跳过，重复 src 去重。
	targetHost := hostnameOf(target.URL)
	seenFrames := map[string]bool{}
	for _, m := range reIFrameSrc.FindAllStringSubmatchIndex(html, -1) {
		raw := html[m[2]:m[3]]
		line := strings.Count(html[:m[0]], "\n") + 1
		resolved := resolveURL(target.URL, raw)
		srcURL, perr := url.Parse(resolved)
		if perr != nil || srcURL.Host == "" && !strings.HasPrefix(raw, "http") {
			continue
		}
		switch {
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "javascript"):
			findings = append(findings, Finding{
				Type: "javascript_protocol", Severity: SeverityMedium,
				Title:       "检测到 javascript: 协议 iframe",
				Description: fmt.Sprintf("iframe src 使用 javascript: 协议（第 %d 行），存在脚本注入风险", line),
				URL:         target.URL, LineNo: line,
				Confidence: 0.8,
				Extra:      map[string]any{"src": raw, "line": line},
			})
			continue
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "data"):
			if reDataURI.MatchString(raw) {
				findings = append(findings, Finding{
					Type: "data_uri", Severity: SeverityLow,
					Title:       "检测到危险 data: URI 嵌入",
					Description: fmt.Sprintf("iframe 嵌入可执行类型的 data: URI（第 %d 行），需人工复核", line),
					URL:         target.URL, LineNo: line,
					Confidence: 0.6,
					Extra:      map[string]any{"src": raw, "line": line},
				})
			}
			continue
		}
		lower := strings.ToLower(srcURL.Scheme)
		if lower != "http" && lower != "https" || sameSite(srcURL.Host, targetHost) {
			continue
		}
		if seenFrames[srcURL.String()] {
			continue
		}
		seenFrames[srcURL.String()] = true
		findings = append(findings, Finding{
			Type: "external_iframe", Severity: SeverityHigh,
			Title:       fmt.Sprintf("外部 iframe 嵌入：%s", raw),
			Description: fmt.Sprintf("页面在第 %d 行嵌入了跨站 iframe：%s", line, resolved),
			URL:         target.URL, LineNo: line,
			Confidence:  0.65,
			Extra:       map[string]any{"iframe_src": resolved, "line": line},
		})
	}

	jsSamples := []string{}
	dataSamples := []string{}
	jsCount := 0
	firstJSLine := 0
	firstDataLine := 0
	for _, m := range reAnchorHref.FindAllStringSubmatchIndex(html, -1) {
		href := html[m[2]:m[3]]
		line := strings.Count(html[:m[0]], "\n") + 1
		switch {
		case reJSProto.MatchString(href):
			jsCount++
			if len(jsSamples) < maxProtocolSamples {
				jsSamples = append(jsSamples, fmt.Sprintf("%s (L%d)", truncate(href, 60), line))
			}
			if firstJSLine == 0 {
				firstJSLine = line
			}
		case reDataURI.MatchString(href):
			if len(dataSamples) < maxProtocolSamples {
				dataSamples = append(dataSamples, fmt.Sprintf("%s (L%d)", truncate(href, 60), line))
			}
			if firstDataLine == 0 {
				firstDataLine = line
			}
		}
	}
	if len(jsSamples) > 0 {
		findings = append(findings, Finding{
			Type: "javascript_protocol", Severity: SeverityMedium,
			Title:       "检测到 javascript: 协议链接",
			Description: fmt.Sprintf("页面包含 javascript:/vbscript: 协议超链接共 %d 处，如：%s", jsCount, strings.Join(jsSamples, "；")),
			URL:         target.URL, LineNo: firstJSLine,
			Confidence: 0.7,
			Extra:      map[string]any{"samples": jsSamples, "first_line": firstJSLine},
		})
	}
	if len(dataSamples) > 0 {
		findings = append(findings, Finding{
			Type: "data_uri", Severity: SeverityLow,
			Title:       "检测到 data: URI 引用",
			Description: fmt.Sprintf("页面锚点引用可执行类型 data: URI 共 %d 处，如：%s", len(dataSamples), strings.Join(dataSamples, "；")),
			URL:         target.URL, LineNo: firstDataLine,
			Confidence: 0.55,
			Extra:      map[string]any{"samples": dataSamples, "first_line": firstDataLine},
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

func hostnameOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func sameSite(a, b string) bool {
	hostA := strings.TrimPrefix(strings.ToLower(hostnameLabel(a)), "www.")
	hostB := strings.TrimPrefix(strings.ToLower(hostnameLabel(b)), "www.")
	return hostA != "" && hostA == hostB
}

func hostnameLabel(hostport string) string {
	host, _, _ := strings.Cut(hostport, ":")
	return host
}
