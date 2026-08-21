package engines

import (
	"context"
	"regexp"
	"strings"
)

const NameContentSecurity = "content_security"
const NameSensitiveWord = "sensitive_word"
const TypeSensitiveWord = "sensitive_word"

// SensitiveWordEngine 敏感词监测引擎（content_security 子能力）：
// 内置敏感词库（涉黄赌毒政）正则匹配页面正文/HTML/标题 + 白名单剔除。
type SensitiveWordEngine struct {
	builtin []*wordRule
}

// wordRule 一条敏感词规则（名称 + 正则）。
type wordRule struct {
	name  string
	group string
	re    *regexp.Regexp
}

// NewSensitiveWordEngine 构造敏感词引擎（内置词库）。
func NewSensitiveWordEngine() *SensitiveWordEngine {
	return &SensitiveWordEngine{builtin: builtinWordRules()}
}

// Name 返回引擎名（content_security 子能力）。
func (e *SensitiveWordEngine) Name() string { return NameSensitiveWord }

// Enabled 策略开关：sensitive_word 未显式关闭即启用（fallback content_security）。
func (e *SensitiveWordEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	if en, ok := p.Enabled[NameSensitiveWord]; ok {
		return en
	}
	en, ok := p.Enabled[NameContentSecurity]
	if !ok {
		return true
	}
	return en
}

// Run 契约占位：敏感词匹配需页面正文，由 Match 提供；此处返回空。
func (e *SensitiveWordEngine) Run(ctx context.Context, target Target, _ Policy) ([]Finding, error) {
	return nil, nil
}

// Match 对给定文本执行敏感词匹配，返回命中 finding。
// source: 页面 URL；whitelist: 组织白名单词汇（kind=content_whitelist）。
func (e *SensitiveWordEngine) Match(source, title, content string, whitelist []string) []Finding {
	var out []Finding
	text := content
	if title != "" && !strings.Contains(content, title) {
		text = title + "\n" + content
	}
	for _, wr := range e.builtin {
		for _, m := range wr.re.FindAllString(text, 5) {
			if containsWhitelist(whitelist, m, text) {
				continue
			}
			out = append(out, Finding{
				Type: TypeSensitiveWord, Severity: SeverityHigh,
				Title:       "命中敏感词: " + wr.name,
				Description: "文本命中违禁词「" + wr.name + "」，命中词: " + truncate(m, 60) + "，来源: regex",
				URL:         source, Confidence: 0.9,
				Extra: map[string]any{"word": wr.name, "group": wr.group, "match": truncate(m, 120), "source": "regex"},
			})
		}
	}
	return out
}

// AddBuiltin 追加自定义规则（敏感词库扩展）。
func (e *SensitiveWordEngine) AddBuiltin(name, group, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	e.builtin = append(e.builtin, &wordRule{name: name, group: group, re: re})
	return nil
}

// containsWhitelist 命中文本片段包含白名单词汇时剔除。
func containsWhitelist(whitelist []string, match, content string) bool {
	for _, w := range whitelist {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if strings.HasPrefix(w, "regex:") {
			re, err := regexp.Compile(w[6:])
			if err == nil && (re.MatchString(content) || re.MatchString(match)) {
				return true
			}
			continue
		}
		if strings.Contains(content, w) || strings.Contains(match, w) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// builtinWordRules 内置敏感词库（涉黄赌毒政，正则匹配）。
func builtinWordRules() []*wordRule {
	pairs := []struct{ name, group, pat string }{
		{"赌博", "gamble", `赌博|百家乐|彩票内幕|棋牌返水|时时彩`},
		{"博彩", "gamble", `博彩|外围投注|下注|赔率`},
		{"毒品", "drug", `冰毒|海洛因|大麻|摇头丸|可卡因|麻黄碱`},
		{"枪支", "gun", `枪支|弹药|仿真枪|气枪|自制炸药`},
		{"色情", "porn", `色情|淫秽|裸聊|成人视频|情色`},
		{"诈骗", "fraud", `刷单|兼职刷信誉|高额回报|稳赚不赔|杀猪盘`},
		{"伪证", "fraud", `办证|假证|代开发票|学历证书办理`},
	}
	out := make([]*wordRule, 0, len(pairs))
	for _, p := range pairs {
		if re, err := regexp.Compile(p.pat); err == nil {
			out = append(out, &wordRule{name: p.name, group: p.group, re: re})
		}
	}
	return out
}
