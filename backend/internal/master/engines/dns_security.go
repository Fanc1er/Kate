package engines

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const NameDNSSecurity = "dns_security"

// DNSResolver DNS 解析器抽象，支持多节点解析对比与污染检测。
type DNSResolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

// systemResolver 使用系统默认解析器。
type systemResolver struct{}

func (systemResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// DNSSecurityEngine DNS 安全引擎：解析检查、多节点解析对比与证书监测。
type DNSSecurityEngine struct {
	resolvers []DNSResolver
}

// NewDNSSecurityEngine 构造 DNS 安全引擎（默认单节点系统解析器）。
func NewDNSSecurityEngine() *DNSSecurityEngine {
	return &DNSSecurityEngine{
		resolvers: []DNSResolver{systemResolver{}},
	}
}

// WithResolvers 注入多个 DNS 解析器，启用多节点解析对比（如公共 DNS 服务器）。
func (e *DNSSecurityEngine) WithResolvers(rs ...DNSResolver) *DNSSecurityEngine {
	if len(rs) == 0 {
		return e
	}
	e.resolvers = rs
	return e
}

// Name 返回引擎名。
func (e *DNSSecurityEngine) Name() string { return NameDNSSecurity }

// Enabled 策略开关：dns_security 未显式关闭即启用。
func (e *DNSSecurityEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameDNSSecurity]
	if !ok {
		return true
	}
	return en
}

// Run 执行 DNS 安全检测。
func (e *DNSSecurityEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	u, err := url.Parse(target.URL)
	if err != nil {
		return findings, nil
	}
	host := u.Hostname()
	if host == "" {
		return findings, nil
	}

	// 多节点解析对比：收集各解析器结果。
	var results [][]net.IP
	for i, r := range e.resolvers {
		ips, err := r.LookupIP(ctx, host)
		if err != nil {
			if i == 0 {
				findings = append(findings, Finding{
					Type:        "dns_resolution_failed",
					Severity:    SeverityMedium,
					Title:       "DNS 解析失败",
					Description: fmt.Sprintf("无法解析主机 %s：%v", host, err),
					URL:         target.URL,
					Confidence:  0.9,
					Extra: map[string]any{
						"host": host,
					},
				})
				return findings, nil
			}
			continue
		}
		if len(ips) == 0 {
			continue
		}
		results = append(results, ips)
	}

	if len(results) == 0 {
		findings = append(findings, Finding{
			Type:        "dns_no_records",
			Severity:    SeverityMedium,
			Title:       "DNS 无记录",
			Description: fmt.Sprintf("主机 %s 无 IP 记录返回", host),
			URL:         target.URL,
			Confidence:  0.8,
		})
		return findings, nil
	}

	// 基于主解析结果检测内网地址。
	for _, ip := range results[0] {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			findings = append(findings, Finding{
				Type:        "dns_internal_ip",
				Severity:    SeverityHigh,
				Title:       "DNS 解析至内网地址",
				Description: fmt.Sprintf("主机 %s 解析到内网/回环 IP：%s，可能存在 DNS 劫持或配置错误", host, ip.String()),
				URL:         target.URL,
				Confidence:  0.85,
				Extra: map[string]any{
					"ip": ip.String(),
				},
			})
		}
	}

	// 多节点解析结果不一致 → 疑似 DNS 污染/劫持。
	if len(results) >= 2 {
		inconsistent := false
		for i := 1; i < len(results); i++ {
			if !sameIPSet(results[0], results[i]) {
				inconsistent = true
				break
			}
		}
		if inconsistent {
			var detail []string
			for _, ips := range results {
				var parts []string
				for _, ip := range ips {
					parts = append(parts, ip.String())
				}
				detail = append(detail, strings.Join(parts, ","))
			}
			findings = append(findings, Finding{
				Type:        "dns_resolver_inconsistent",
				Severity:    SeverityHigh,
				Title:       "多节点 DNS 解析结果不一致",
				Description: fmt.Sprintf("主机 %s 在不同解析节点返回不同 IP，疑似 DNS 污染或劫持", host),
				URL:         target.URL,
				Confidence:  0.7,
				Extra: map[string]any{
					"results": detail,
				},
			})
		}
	}

	// 证书监测（有效期/SAN），仅对 HTTPS 目标；复用 phishing 的证书校验。
	if u.Scheme == "https" {
		addr := u.Host
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, "443")
		}
		certFindings := checkTLSCertificate(addr, host)
		for i := range certFindings {
			certFindings[i].URL = target.URL
		}
		findings = append(findings, certFindings...)
	}

	return findings, nil
}

// sameIPSet 比较两个 IP 集合是否一致（顺序无关）。
func sameIPSet(a, b []net.IP) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, ip := range a {
		set[ip.String()] = true
	}
	for _, ip := range b {
		if !set[ip.String()] {
			return false
		}
	}
	return true
}

// extractHost 从 URL 中提取主机名。
func extractHost(rawURL string) string {
	host := rawURL
	// 去掉协议
	if strings.HasPrefix(host, "http://") {
		host = host[7:]
	} else if strings.HasPrefix(host, "https://") {
		host = host[8:]
	}
	// 去掉路径
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	// 去掉端口
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	return host
}
