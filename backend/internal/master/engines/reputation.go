package engines

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/url"
	"strings"
)

const NameReputation = "reputation"

// ReputationFetcher 外部威胁情报源接口。
// 完整实现可接入 VirusTotal、Shodan、Whois 等 API。
type ReputationFetcher interface {
	// Lookup 返回主机威胁评分（0~100），越高越可疑。
	Lookup(ctx context.Context, host string) (int, error)
}

// ReputationEngine 信誉监测引擎：本地启发式 + 可插拔外部威胁情报源。
type ReputationEngine struct {
	fetcher ReputationFetcher
}

// NewReputationEngine 构造信誉监测引擎。
func NewReputationEngine() *ReputationEngine { return &ReputationEngine{} }

// WithReputationFetcher 注入外部威胁情报源。
func (e *ReputationEngine) WithReputationFetcher(f ReputationFetcher) *ReputationEngine {
	e.fetcher = f
	return e
}

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

// Run 执行信誉监测。
func (e *ReputationEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	u, err := url.Parse(target.URL)
	if err != nil {
		return findings, nil
	}
	host := u.Hostname()
	if host == "" {
		return findings, nil
	}

	// 已知恶意域名模式检测
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

	// 外部威胁情报源评分（0~100，>=60 视为高威胁）。
	if e.fetcher != nil {
		score, ferr := e.fetcher.Lookup(ctx, host)
		if ferr == nil && score >= 60 {
			findings = append(findings, Finding{
				Type:        "intel_threat_score",
				Severity:    SeverityHigh,
				Title:       "外部情报威胁评分过高",
				Description: fmt.Sprintf("威胁情报源对 %s 评分为 %d/100", host, score),
				URL:         target.URL,
				Confidence:  0.8,
				Extra: map[string]any{
					"score": score,
				},
			})
		}
	}

	// 高熵随机化域名（算法生成域名常被用于恶意活动）。
	if randomLookingDomain(host) {
		findings = append(findings, Finding{
			Type:        "suspicious_domain_entropy",
			Severity:    SeverityMedium,
			Title:       "检测到高熵随机域名",
			Description: fmt.Sprintf("域名主体 %s 字符分布高度随机，疑似算法生成域名", host),
			URL:         target.URL,
			Confidence:  0.5,
			Extra: map[string]any{
				"entropy": domainEntropy(host),
			},
		})
	}

	// 纯 IP 直连（跳过域名，常被用于钓鱼/恶意站点托管）。
	if ip := net.ParseIP(host); ip != nil {
		findings = append(findings, Finding{
			Type:        "direct_ip_host",
			Severity:    SeverityInfo,
			Title:       "目标使用 IP 直连",
			Description: fmt.Sprintf("目标以 IP %s 直连而非域名，建议核实归属与用途", host),
			URL:         target.URL,
			Confidence:  0.4,
			Extra: map[string]any{
				"ip": ip.String(),
			},
		})
	}

	return findings, nil
}

// randomLookingDomain 判断域名主体是否随机化（熵高且长度足够）。
func randomLookingDomain(host string) bool {
	label := host
	if idx := strings.Index(label, "."); idx >= 0 {
		label = label[:idx]
	}
	if len(label) < 8 {
		return false
	}
	return domainEntropy(strings.ToLower(label)) >= 0.6
}

// domainEntropy 计算字符串的香农熵（相对 36 进制字符集归一化到 0~1）。
func domainEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	freq := make(map[byte]int)
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	var ent float64
	for _, c := range freq {
		p := float64(c) / float64(len(s))
		ent -= p * math.Log2(p)
	}
	// 归一化基准：36 进制（26 字母 + 10 数字）最大熵。
	maxEnt := math.Log2(36)
	if maxEnt == 0 {
		return 0
	}
	return ent / maxEnt
}
