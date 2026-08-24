package engines

import (
	"context"
	"fmt"
	"strings"
)

const NameReputation = "reputation"

// ReputationEngine 信誉监测引擎骨架：IP/域名威胁情报查询。
// 完整实现需接入外部威胁情报 API（如 VirusTotal、Shodan、Whois 等）。
type ReputationEngine struct{}

// NewReputationEngine 构造信誉监测引擎。
func NewReputationEngine() *ReputationEngine { return &ReputationEngine{} }

// Name 返回引擎名。
func (e *ReputationEngine) Name() string { return NameReputation }

// Enabled 策略开关：reputation 未显式关闭即启用。
func (e *ReputationEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameReputation]
	if !ok {
		return true
	}
	return en
}

// Run 执行信誉监测（骨架：基于已知威胁域名列表）。
func (e *ReputationEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	// 骨架实现：检查目标 URL 是否在已知威胁域名列表中
	// 完整实现需调用威胁情报 API
	host := target.URL
	for i, c := range host {
		if c == ':' || c == '/' {
			host = host[:i]
			break
		}
	}

	// 已知恶意域名模式检测（骨架）
	maliciousPatterns := []string{
		"malware", "phishing", "botnet", "ransomware",
		"exploit", "c2", "command-control", "darkweb",
	}
	for _, pattern := range maliciousPatterns {
		if strings.Contains(strings.ToLower(host), pattern) {
			findings = append(findings, Finding{
				Type:        "threat_domain",
				Severity:    SeverityHigh,
				Title:       fmt.Sprintf("威胁域名匹配：%s", pattern),
				Description: fmt.Sprintf("目标域名 %s 匹配已知威胁模式：%s", host, pattern),
				URL:         target.URL,
				Confidence:  0.6,
				Extra: map[string]any{
					"pattern": pattern,
				},
			})
			break
		}
	}

	return findings, nil
}
