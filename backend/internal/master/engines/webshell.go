package engines

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const NameWebshell = "webshell"

var (
	reEval       = regexp.MustCompile(`(?i)\beval\s*\(`)
	reBase64Decode = regexp.MustCompile(`(?i)base64_decode\s*\(`)
	reAssert     = regexp.MustCompile(`(?i)\bassert\s*\(`)
	reSystem     = regexp.MustCompile(`(?i)\b(system|passthru|exec|shell_exec|pcntl_exec|popen|proc_open)\s*\(`)
	reWrite      = regexp.MustCompile(`(?i)(fputs|fwrite|file_put_contents)\s*\(`)
	reEvalPattern = regexp.MustCompile(`(?i)(eval|assert|preg_replace|call_user_func)\s*\(\s*(base64_decode|gzinflate|gzuncompress|gzdecode|str_rot13|pack|chr)\s*\(`)
)

// WebshellPatterns 常见 Webshell 特征模式。
var WebshellPatterns = []struct {
	pattern *regexp.Regexp
	severity string
	desc     string
}{
	{reEval, SeverityHigh, "eval 函数调用"},
	{reBase64Decode, SeverityMedium, "base64_decode 函数调用"},
	{reAssert, SeverityHigh, "assert 函数调用"},
	{reSystem, SeverityCritical, "系统命令执行函数"},
	{reWrite, SeverityHigh, "文件写入函数"},
}

// WebshellEngine Webshell 检测引擎骨架：基于特征码与模式匹配检测后门文件。
// 完整实现需接入路径枚举与流量特征分析。
type WebshellEngine struct{}

// NewWebshellEngine 构造 Webshell 检测引擎。
func NewWebshellEngine() *WebshellEngine { return &WebshellEngine{} }

// Name 返回引擎名。
func (e *WebshellEngine) Name() string { return NameWebshell }

// Enabled 策略开关：webshell 未显式关闭即启用。
func (e *WebshellEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameWebshell]
	if !ok {
		return true
	}
	return en
}

// Run 执行 Webshell 检测（骨架：基于模式匹配检查代码内容）。
func (e *WebshellEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	// 骨架实现：检测目标 URL 返回内容中的危险模式
	// 完整实现需在 Worker 侧对文件内容进行扫描
	var findings []Finding
	// TODO: 接入文件内容扫描逻辑（Worker 侧实现）
	// 此处返回空 findings，等待后续迭代补充
	return findings, nil
}

// CheckContent 对给定代码内容执行 Webshell 特征检测。
func CheckContent(code string) []Finding {
	var findings []Finding
	for _, pat := range WebshellPatterns {
		if pat.pattern.MatchString(code) {
			findings = append(findings, Finding{
				Type:        "webshell_pattern",
				Severity:    pat.severity,
				Title:       fmt.Sprintf("检测到 Webshell 特征：%s", pat.desc),
				Description: fmt.Sprintf("代码内容匹配危险模式：%s", pat.desc),
				URL:         "",
				Confidence:  0.7,
				Extra: map[string]any{
					"pattern": pat.desc,
				},
			})
		}
	}
	// 检测混淆 eval 模式
	if reEvalPattern.MatchString(code) {
		findings = append(findings, Finding{
			Type:        "webshell_obfuscated",
			Severity:    SeverityCritical,
			Title:       "检测到混淆 Webshell 模式",
			Description: "代码包含 eval/assert + 编码函数组合，高度疑似 Webshell",
			URL:         "",
			Confidence:  0.9,
		})
	}
	// 检测常见 Webshell 文件名模式
	lower := strings.ToLower(code)
	if strings.Contains(lower, "<?php") && (strings.Contains(lower, "eval") || strings.Contains(lower, "system") || strings.Contains(lower, "exec")) {
		findings = append(findings, Finding{
			Type:        "webshell_php",
			Severity:    SeverityCritical,
			Title:       "检测到 PHP Webshell",
			Description: "PHP 代码包含危险函数组合，疑似后门文件",
			URL:         "",
			Confidence:  0.85,
		})
	}
	return findings
}
