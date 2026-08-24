package engines

import (
	"context"
	"fmt"
	"net"
	"strings"
)

const NameDNSSecurity = "dns_security"

// DNSSecurityEngine DNS 安全引擎骨架：解析检查与证书监测。
// 完整实现需支持多节点解析对比、污染检测、CRL-OCSP 撤销校验。
type DNSSecurityEngine struct{}

// NewDNSSecurityEngine 构造 DNS 安全引擎。
func NewDNSSecurityEngine() *DNSSecurityEngine { return &DNSSecurityEngine{} }

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
	host := extractHost(target.URL)

	// DNS 解析检查
	ips, err := net.LookupIP(host)
	if err != nil {
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

	if len(ips) == 0 {
		findings = append(findings, Finding{
			Type:        "dns_no_records",
			Severity:    SeverityMedium,
			Title:       "DNS 无记录",
			Description: fmt.Sprintf("主机 %s 无 IP 记录返回", host),
			URL:         target.URL,
			Confidence:  0.8,
		})
	}

	// 检测 IP 是否为内网地址
	for _, ip := range ips {
		if ip.IsPrivate() {
			findings = append(findings, Finding{
				Type:        "dns_internal_ip",
				Severity:    SeverityHigh,
				Title:       "DNS 解析至内网地址",
				Description: fmt.Sprintf("主机 %s 解析到内网 IP：%s，可能存在 DNS 劫持或配置错误", host, ip.String()),
				URL:         target.URL,
				Confidence:  0.85,
				Extra: map[string]any{
					"ip": ip.String(),
				},
			})
		}
	}

	// TODO: 多节点解析对比
	// TODO: 证书监测（有效期/CRL-OCSP/SAN）

	return findings, nil
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
