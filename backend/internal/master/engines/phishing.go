package engines

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const NamePhishing = "phishing"

// knownBrands 常见被仿冒的品牌域名（顶级注册域名，用于 typosquatting 比对）。
var knownBrands = []string{
	"google.com", "facebook.com", "paypal.com", "apple.com",
	"microsoft.com", "amazon.com", "taobao.com", "jd.com",
	"weibo.com", "alipay.com", "bankofamerica.com", "chase.com",
	"wellsfargo.com", "icbc.com.cn", "baidu.com", "qq.com",
	"163.com", "outlook.com", "linkedin.com", "netflix.com",
}

// PhishingEngine 钓鱼检测引擎：基于域名相似度与证书异常检测钓鱼站点。
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
	u, err := url.Parse(target.URL)
	if err != nil {
		return findings, nil
	}
	domain := u.Hostname()

	// 检测 HTTPS 使用情况（HTTP 明文存在中间人攻击风险）
	if u.Scheme == "http" {
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

	// 检测可疑域名特征（数字/连字符过多）
	if isSuspiciousDomain(domain) {
		findings = append(findings, Finding{
			Type:        "suspicious_domain",
			Severity:    SeverityMedium,
			Title:       "检测到可疑域名",
			Description: fmt.Sprintf("域名 %s 包含可疑特征（数字/连字符过多）", domain),
			URL:         target.URL,
			Confidence:  0.5,
		})
	}

	// 检测 typosquatting：与知名品牌域名计算编辑距离
	if brand, dist := detectTyposquatting(domain); brand != "" {
		findings = append(findings, Finding{
			Type:        "typosquatting",
			Severity:    SeverityHigh,
			Title:       fmt.Sprintf("疑似仿冒域名：%s", brand),
			Description: fmt.Sprintf("域名 %s 与知名品牌 %s 的编辑距离为 %d，疑似 typosquatting 钓鱼站点", domain, brand, dist),
			URL:         target.URL,
			Confidence:  0.7,
			Extra: map[string]any{
				"brand":    brand,
				"distance": dist,
			},
		})
	}

	// 检测 HTTPS 证书有效期与域名覆盖
	if u.Scheme == "https" {
		addr := u.Host
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, "443")
		}
		certFindings := checkTLSCertificate(addr, domain)
		for i := range certFindings {
			certFindings[i].URL = target.URL
		}
		findings = append(findings, certFindings...)
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

// isSuspiciousDomain 判断域名是否可疑。
func isSuspiciousDomain(domain string) bool {
	// 检查域名中数字比例过高（>50%）
	digitCount := 0
	for _, c := range domain {
		if c >= '0' && c <= '9' {
			digitCount++
		}
	}
	if len(domain) > 0 && digitCount*2 > len(domain) {
		return true
	}
	// 检查过多连字符
	hyphenCount := strings.Count(domain, "-")
	if hyphenCount > 5 {
		return true
	}
	return false
}

// detectTyposquatting 检测目标域名是否为知名品牌的仿冒域名，返回命中的品牌与编辑距离。
func detectTyposquatting(domain string) (string, int) {
	d := domain
	d = strings.TrimPrefix(strings.TrimPrefix(d, "www."), "www.")
	// 去掉端口
	if idx := strings.Index(d, ":"); idx >= 0 {
		d = d[:idx]
	}
	if d == "" {
		return "", 0
	}

	// 与品牌列表计算编辑距离，距离 <=2 视为高度可疑（含 0→o 等字符替换）。
	best, bestDist := "", 4
	for _, brand := range knownBrands {
		if d == brand {
			continue
		}
		if dist := levenshtein(d, brand); dist < bestDist {
			bestDist = dist
			best = brand
		}
	}
	if bestDist <= 2 {
		return best, bestDist
	}

	// 品牌名作为子串出现（如 paypa1-secure.com / google-verify.com）
	for _, brand := range knownBrands {
		if d != brand && strings.Contains(d, brand) {
			return brand, 2
		}
	}
	return "", 0
}

// levenshtein 计算两个字符串的编辑距离（迭代 DP，O(m*n)）。
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur := make([]int, lb+1)
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = minInt(prev[j]+1, minInt(cur[j-1]+1, prev[j-1]+cost))
		}
		prev = cur
	}
	return prev[lb]
}

// minInt 返回较小整数。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// checkTLSCertificate 建立 TLS 连接读取服务器证书并校验有效期与域名覆盖。
func checkTLSCertificate(addr, hostname string) []Finding {
	var findings []Finding
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil
	}
	cert := state.PeerCertificates[0]
	now := time.Now()

	// 证书有效期检查
	if now.Before(cert.NotBefore) {
		findings = append(findings, Finding{
			Type:        "cert_not_yet_valid",
			Severity:    SeverityHigh,
			Title:       "证书尚未生效",
			Description: fmt.Sprintf("证书有效期开始于 %s，当前时间 %s", cert.NotBefore.Format(time.RFC3339), now.Format(time.RFC3339)),
			Confidence:  0.9,
		})
	}
	if now.After(cert.NotAfter) {
		findings = append(findings, Finding{
			Type:        "cert_expired",
			Severity:    SeverityHigh,
			Title:       "证书已过期",
			Description: fmt.Sprintf("证书已于 %s 过期", cert.NotAfter.Format(time.RFC3339)),
			Confidence:  0.9,
		})
	} else if cert.NotAfter.Sub(now) < 30*24*time.Hour {
		findings = append(findings, Finding{
			Type:        "cert_expiring_soon",
			Severity:    SeverityLow,
			Title:       "证书即将过期",
			Description: fmt.Sprintf("证书将于 %s 过期，剩余 %s", cert.NotAfter.Format(time.RFC3339), cert.NotAfter.Sub(now).Round(time.Hour)),
			Confidence:  0.7,
		})
	}

	// 域名覆盖检查
	if err := cert.VerifyHostname(hostname); err != nil {
		findings = append(findings, Finding{
			Type:        "cert_hostname_mismatch",
			Severity:    SeverityHigh,
			Title:       "证书域名不匹配",
			Description: fmt.Sprintf("证书不覆盖主机名 %s：%v", hostname, err),
			Confidence:  0.9,
		})
	}

	return findings
}
