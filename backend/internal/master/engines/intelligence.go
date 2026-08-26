package engines

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const NameIntelligence = "intelligence"

// IntelItem 单条安全情报。
type IntelItem struct {
	ID          string // CVE-YYYY-NNNN / CNVD-... / CNNVD-...
	Title       string
	Description string
	Severity    string
}

// IntelligenceProvider 外部安全情报源接口。
type IntelligenceProvider interface {
	// Query 查询命中目标的情报条目。
	Query(ctx context.Context, target Target) ([]IntelItem, error)
}

// intelRule 内置组件版本匹配规则。
type intelRule struct {
	Component  string
	MaxVersion string // version <= MaxVersion 时命中（简化区间）
	Item       IntelItem
}

// intelRules 内置精简情报规则表（演示用）。
// 完整实现应定时从 CVE/CNVD/CNNVD 订阅源同步到 intel_items 表。
var intelRules = []intelRule{
	{
		Component:  "nginx",
		MaxVersion: "1.18.0",
		Item: IntelItem{
			ID:          "CVE-2021-23017",
			Title:       "nginx DNS 解析器 off-by-one 漏洞",
			Description: "nginx 解析器存在 off-by-one 越界写入，可导致 worker 崩溃或内存破坏",
			Severity:    SeverityCritical,
		},
	},
	{
		Component:  "nginx",
		MaxVersion: "1.24.0",
		Item: IntelItem{
			ID:          "CVE-2023-44487",
			Title:       "HTTP/2 Rapid Reset DoS",
			Description: "HTTP/2 快速重置攻击可对服务造成拒绝服务",
			Severity:    SeverityHigh,
		},
	},
	{
		Component:  "apache",
		MaxVersion: "2.4.55",
		Item: IntelItem{
			ID:          "CVE-2023-25690",
			Title:       "Apache HTTP Server 请求走私",
			Description: "Apache 2.4.55 及以下版本存在 HTTP 请求走私漏洞",
			Severity:    SeverityHigh,
		},
	},
}

// IntelligenceEngine 安全情报引擎：内置组件版本匹配 + 可插拔外部情报源。
type IntelligenceEngine struct {
	client    *http.Client
	providers []IntelligenceProvider
}

// NewIntelligenceEngine 构造安全情报引擎。
func NewIntelligenceEngine() *IntelligenceEngine {
	return &IntelligenceEngine{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// WithProvider 注入外部安全情报源。
func (e *IntelligenceEngine) WithProvider(p IntelligenceProvider) *IntelligenceEngine {
	e.providers = append(e.providers, p)
	return e
}

// Name 返回引擎名。
func (e *IntelligenceEngine) Name() string { return NameIntelligence }

// Enabled 策略开关：intelligence 未显式关闭即启用。
func (e *IntelligenceEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameIntelligence]
	if !ok {
		return true
	}
	return en
}

// Run 执行安全情报检测。
func (e *IntelligenceEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding

	// 获取目标响应头，识别组件版本并匹配内置规则。
	if ctx.Err() == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
		if err == nil {
			if resp, err := e.client.Do(req); err == nil {
				server := resp.Header.Get("Server")
				_ = resp.Body.Close()
				component := detectComponent(server)
				version := ExtractVersion(server)
				for _, item := range matchIntelRules(component, version) {
					findings = append(findings, MatchCVE(target.URL, item.ID, item.Description))
				}
			}
		}
	}

	// 外部情报源。
	for _, prov := range e.providers {
		items, err := prov.Query(ctx, target)
		if err != nil {
			continue
		}
		for _, item := range items {
			findings = append(findings, MatchCVE(target.URL, item.ID, item.Description))
		}
	}

	return findings, nil
}

// matchIntelRules 按组件与版本匹配内置情报规则。
func matchIntelRules(component, version string) []IntelItem {
	var out []IntelItem
	if component == "" || version == "" {
		return out
	}
	for _, r := range intelRules {
		if r.Component == component && compareVersions(version, r.MaxVersion) <= 0 {
			out = append(out, r.Item)
		}
	}
	return out
}

// detectComponent 从 Server 响应头中识别 Web 组件。
func detectComponent(server string) string {
	lower := strings.ToLower(server)
	for _, c := range []string{"openresty", "nginx", "apache", "iis", "tomcat", "jetty"} {
		if strings.Contains(lower, c) {
			return c
		}
	}
	return ""
}

// compareVersions 按点分数字段比较版本号，a<b 返回 -1，a==b 返回 0，a>b 返回 1。
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// MatchCVE 检查目标是否命中指定 CVE。
func MatchCVE(targetURL, cveID, description string) Finding {
	return Finding{
		Type:        "cve_match",
		Severity:    SeverityHigh,
		Title:       fmt.Sprintf("命中 CVE 情报：%s", cveID),
		Description: fmt.Sprintf("资产 %s 命中安全情报 %s：%s", targetURL, cveID, description),
		URL:         targetURL,
		Confidence:  0.9,
		Extra: map[string]any{
			"cve_id":      cveID,
			"description": description,
		},
	}
}

// ExtractVersion 从目标 URL 或 User-Agent 中提取版本号（骨架）。
func ExtractVersion(raw string) string {
	parts := strings.Split(raw, "/")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		for i := 0; i < len(last); i++ {
			if last[i] == ' ' || last[i] == '(' || last[i] == ')' {
				last = last[:i]
				break
			}
		}
		if len(last) > 0 && last[0] >= '0' && last[0] <= '9' {
			return last
		}
	}
	return ""
}
