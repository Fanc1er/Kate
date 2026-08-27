package engines

import (
	"context"
	"fmt"
	"strings"
)

// RuleReputationFetcher 基于平台恶意域名规则库的本地信誉源。
// 规则来自 rule_definitions(kind=malicious_domain)，随任务 domain_rules 下发。
// 支持 "example.com" 精确匹配与 ".example.com" / "*.example.com" 子域后缀匹配，
// 命中返回高分评分，未命中返回 0。
type RuleReputationFetcher struct {
	exact map[string]bool
	sufix []string
}

// NewRuleReputationFetcher 构建本地信誉源，patterns 为恶意域名规则列表。
func NewRuleReputationFetcher(patterns []string) *RuleReputationFetcher {
	f := &RuleReputationFetcher{exact: make(map[string]bool, len(patterns))}
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		switch {
		case strings.HasPrefix(p, "*."):
			f.sufix = append(f.sufix, strings.TrimPrefix(p, "*."))
		case strings.HasPrefix(p, "."):
			f.sufix = append(f.sufix, strings.TrimPrefix(p, "."))
		default:
			f.exact[p] = true
		}
	}
	return f
}

// Lookup 返回主机威胁评分：命中恶意域名库返回 95，否则 0。
func (f *RuleReputationFetcher) Lookup(_ context.Context, host string) (int, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || (f.exact == nil && len(f.sufix) == 0) {
		return 0, nil
	}
	if f.exact[host] {
		return 95, nil
	}
	for _, root := range f.sufix {
		if host == root || strings.HasSuffix(host, "."+root) {
			return 90, nil
		}
	}
	return 0, nil
}

// ComponentRule 组件版本漏洞规则（可由外部情报库下发）。
type ComponentRule struct {
	Component   string // 组件名小写：nginx/apache/tomcat...
	MaxVersion  string // version <= MaxVersion 时命中
	ID          string // CVE-YYYY-NNNN 等
	Title       string
	Description string
	Severity    string
}

// HeaderIntelProvider 基于 Server 响应头的本地组件情报源。
// 由 Worker 注入目标响应的 Server 头，Query 时复用内置组件识别与版本比较逻辑，
// 零额外网络请求即可完成组件 CVE 匹配。
type HeaderIntelProvider struct {
	server      string
	extraRules  []ComponentRule
	hasInternal bool
}

// NewHeaderIntelProvider 构建本地组件情报源，server 为目标响应头原值（可为空）。
func NewHeaderIntelProvider(server string) *HeaderIntelProvider {
	return &HeaderIntelProvider{server: server, hasInternal: true}
}

// WithComponentRules 追加情报库下发的组件规则。
func (p *HeaderIntelProvider) WithComponentRules(rules []ComponentRule) *HeaderIntelProvider {
	p.extraRules = append(p.extraRules, rules...)
	return p
}

// Query 按注入的 Server 头匹配组件漏洞规则，返回命中的情报条目。
func (p *HeaderIntelProvider) Query(_ context.Context, _ Target) ([]IntelItem, error) {
	if p.server == "" {
		return nil, nil
	}
	component := detectComponent(p.server)
	version := ExtractVersion(p.server)
	if component == "" || version == "" {
		return nil, nil
	}
	var out []IntelItem
	if p.hasInternal {
		out = append(out, matchIntelRules(component, version)...)
	}
	for _, r := range p.extraRules {
		if strings.EqualFold(r.Component, component) && compareVersions(version, r.MaxVersion) <= 0 {
			sev := r.Severity
			if sev == "" {
				sev = SeverityHigh
			}
			title := r.Title
			if title == "" {
				title = fmt.Sprintf("%s 组件版本命中漏洞", component)
			}
			out = append(out, IntelItem{ID: r.ID, Title: title, Description: r.Description, Severity: sev})
		}
	}
	return out, nil
}
