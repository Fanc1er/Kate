package engines

import (
	"context"
	"fmt"
	"regexp"
)

const (
	// NameSensitiveInfo 敏感信息监测子能力名。
	NameSensitiveInfo = "sensitive_info"
	// TypeSensitiveInfo finding 类型。
	TypeSensitiveInfo = "sensitive_info"
)

// SensitiveRule 一条敏感信息规则。
type SensitiveRule struct {
	Group  string
	Name   string
	Scope  string   // request line / request header / response header / response body
	SRegex *regexp.Regexp // 过滤正则（可空，先命中才提取）
	FRegex *regexp.Regexp // 提取正则
	Sensitive bool
}

// SensitiveHit 单次命中明细。
type SensitiveHit struct {
	Group       string `json:"group"`
	Name        string `json:"name"`
	MatchedText string `json:"matched_text"`
	Scope       string `json:"scope"`
	URL         string `json:"url"`
}

// SensitiveInfoEngine 敏感信息规则集提取引擎（content_security 子能力）：
// 按 scope 分层匹配 request line/header/body，s_regex 过滤 + f_regex 提取。
type SensitiveInfoEngine struct {
	rules []SensitiveRule
}

// NewSensitiveInfoEngine 构造引擎（内置身份证/手机号/邮箱/JWT/Authorization/云凭证规则）。
func NewSensitiveInfoEngine() *SensitiveInfoEngine {
	return &SensitiveInfoEngine{rules: builtinSensitiveRules()}
}

// Name 返回引擎名。
func (e *SensitiveInfoEngine) Name() string { return NameSensitiveInfo }

// Enabled 策略开关：sensitive_info 未显式关闭即启用（fallback content_security）。
func (e *SensitiveInfoEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	if en, ok := p.Enabled[NameSensitiveInfo]; ok {
		return en
	}
	en, ok := p.Enabled[NameContentSecurity]
	if !ok {
		return true
	}
	return en
}

// Run 契约占位：匹配需正文，由 Match 提供。
func (e *SensitiveInfoEngine) Run(ctx context.Context, target Target, _ Policy) ([]Finding, error) {
	return nil, nil
}

// Match 对正文文本执行敏感信息提取。samples: 请求行/头/体 文本快照（按 scope 匹配）。
// 返回主命中 finding 与命中明细。
func (e *SensitiveInfoEngine) Match(pageURL string, samples map[string]string) ([]Finding, []SensitiveHit) {
	var hits []SensitiveHit
	for _, rule := range e.rules {
		text, ok := samples[rule.Scope]
		if !ok {
			// 兼容简化调用：仅 response body。
			if rule.Scope == "response body" || rule.Scope == "response" {
				text = samples["body"]
			}
		}
		if text == "" {
			continue
		}
		// 过滤正则：先命中才提取。
		if rule.SRegex != nil && !rule.SRegex.MatchString(text) {
			continue
		}
		seen := map[string]bool{}
		for _, m := range rule.FRegex.FindAllString(text, 20) {
			if seen[m] {
				continue
			}
			seen[m] = true
			hits = append(hits, SensitiveHit{
				Group: rule.Group, Name: rule.Name, MatchedText: truncate(m, 120),
				Scope: rule.Scope, URL: pageURL,
			})
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}
	f := Finding{
		Type: TypeSensitiveInfo, Severity: SeverityHigh,
		Title:       "敏感信息泄漏",
		Description: fmt.Sprintf("页面发现 %d 条敏感信息命中（身份证/手机号/邮箱/JWT/凭证等）", len(hits)),
		URL:         pageURL, Confidence: 0.9,
		Extra: map[string]any{"sensitive_info_hits": hits, "hit_count": len(hits)},
	}
	return []Finding{f}, hits
}

// AddRule 追加自定义敏感信息规则。
func (e *SensitiveInfoEngine) AddRule(group, name, scope, sRegex, fRegex string, sensitive bool) error {
	var sre *regexp.Regexp
	if sRegex != "" {
		var err error
		sre, err = regexp.Compile(sRegex)
		if err != nil {
			return err
		}
	}
	fre, err := regexp.Compile(fRegex)
	if err != nil {
		return err
	}
	e.rules = append(e.rules, SensitiveRule{
		Group: group, Name: name, Scope: scope, SRegex: sre, FRegex: fre, Sensitive: sensitive,
	})
	return nil
}

// builtinSensitiveRules 内置敏感信息规则集。
func builtinSensitiveRules() []SensitiveRule {
	comp := func(f string) *regexp.Regexp { re, _ := regexp.Compile(f); return re }
	rules := []struct{ group, name, scope, s, f string; sensitive bool }{
		{"身份证", "身份证号码", "response body", "", `\b\d{17}[\dXx]\b`, true},
		{"手机号", "中国大陆手机号", "response body", "", `\b1[3-9]\d{9}\b`, true},
		{"邮箱", "邮箱地址", "response body", "", `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`, true},
		{"JWT", "JWT Token", "response body", "", `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`, true},
		{"Authorization", "Authorization 头", "request header", "", `(?i)bearer\s+[A-Za-z0-9._~+/=-]+`, true},
		{"云凭证", "阿里云 AccessKey", "response body", "", `(?i)LTAI[A-Za-z0-9]{12,}`, true},
		{"云凭证", "AWS AccessKey", "response body", "", `(?i)AKIA[A-Z0-9]{16}`, true},
	}
	out := make([]SensitiveRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, SensitiveRule{
			Group: r.group, Name: r.name, Scope: r.scope,
			SRegex: nil, FRegex: comp(r.f), Sensitive: r.sensitive,
		})
	}
	return out
}
