package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const NamePhishing = "phishing"

// PhishingEngine 钓鱼检测引擎骨架：基于域名相似度与证书异常检测钓鱼站点。
// 完整实现需接入 Levenshtein 距离与证书吊销列表 (CRL/OCSP)。
type PhishingEngine struct {
	client *http.Client
}

// NewPhishingEngine 构造钓鱼检测引擎。
func NewPhishingEngine() *PhishingEngine {
	return &PhishingEngine{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Name 返回引擎名。
func (e *PhishingEngine) Name() string { return NamePhishing }

// Enabled 策略开关：phishing 未显式关闭即启用。
func (e *PhishingEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NamePhishing]
	if !ok {
		return true
	}
	return en
}

// Run 执行钓鱼检测。
func (e *PhishingEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	// 骨架实现：检查目标 URL 的基本钓鱼特征
	u, err := parseDomain(target.URL)
	if err != nil {
		return findings, nil
	}

	// 检测 HTTPS 证书异常（骨架：仅检查是否使用 HTTPS）
	if strings.HasPrefix(target.URL, "http://") {
		findings = append(findings, Finding{
			Type:        "no_https",
			Severity:    SeverityLow,
			Title:       "目标未使用 HTTPS",
			Description: fmt.Sprintf("目标 %s 使用 HTTP 而非 HTTPS，存在中间人攻击风险", target.URL),
			URL:         target.URL,
			Confidence:  0.8,
			Extra: map[string]any{
				"protocol": "http",
			},
		})
	}

	// 检测可疑域名特征（骨架）
	if isSuspiciousDomain(u) {
		findings = append(findings, Finding{
			Type:        "suspicious_domain",
			Severity:    SeverityMedium,
			Title:       "检测到可疑域名",
			Description: fmt.Sprintf("域名 %s 包含可疑特征（数字/连字符过多）", u),
			URL:         target.URL,
			Confidence:  0.5,
		})
	}

	return findings, nil
}

// parseDomain 从 URL 中提取域名。
func parseDomain(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

// isSuspiciousDomain 判断域名是否可疑（骨架实现）。
func isSuspiciousDomain(domain string) bool {
	// 检查域名中数字比例过高
	digitCount := 0
	for _, c := range domain {
		if c >= '0' && c <= '9' {
			digitCount++
		}
	}
	if len(domain) > 0 && digitCount/len(domain) > 3 {
		return true
	}
	// 检查过多连字符
	hyphenCount := strings.Count(domain, "-")
	if hyphenCount > 5 {
		return true
	}
	return false
}
