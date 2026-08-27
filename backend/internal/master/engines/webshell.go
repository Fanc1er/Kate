package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const NameWebshell = "webshell"

var (
	reEval         = regexp.MustCompile(`(?i)\beval\s*\(`)
	reBase64Decode = regexp.MustCompile(`(?i)base64_decode\s*\(`)
	reAssert       = regexp.MustCompile(`(?i)\bassert\s*\(`)
	reSystem       = regexp.MustCompile(`(?i)\b(system|passthru|shell_exec|pcntl_exec|popen|proc_open)\s*\(`)
	reWrite        = regexp.MustCompile(`(?i)\b(fputs|fwrite|file_put_contents)\s*\(`)
	reEvalPattern  = regexp.MustCompile(`(?i)(eval|assert|preg_replace|call_user_func)\s*\(\s*(base64_decode|gzinflate|gzuncompress|gzdecode|str_rot13|pack|chr)\s*\(`)
	reCreateFunc   = regexp.MustCompile(`(?i)\b(create_function|register_shutdown_function|preg_replace\s*\(.*/e)`)
	reRequestVar   = regexp.MustCompile(`(eval|assert|system|exec|shell_exec|passthru)\s*\(\s*(\$_(GET|POST|REQUEST|COOKIE)|php://input)`)
	reDangerFunc   = regexp.MustCompile(`\b(eval|assert|system|exec|shell_exec|passthru)\s*\(`)
)

// WebshellPatterns 常见 Webshell 特征模式。
// phpOnly 的模式在非 PHP 内容（如普通前端 JS）中不判定，避免 eval/assert 误报。
var WebshellPatterns = []struct {
	pattern  *regexp.Regexp
	severity string
	desc     string
	phpOnly  bool
}{
	{reSystem, SeverityCritical, "系统命令执行函数", false},
	{reBase64Decode, SeverityMedium, "base64_decode 函数调用", true},
	{reEval, SeverityHigh, "eval 函数调用", true},
	{reAssert, SeverityHigh, "assert 函数调用", true},
	{reWrite, SeverityHigh, "文件写入函数", true},
	{reCreateFunc, SeverityHigh, "动态函数构造/注册", false},
}

// WebshellEngine Webshell 检测引擎：对目标页面内容执行特征码与混淆模式匹配。
type WebshellEngine struct {
	client *http.Client
}

// NewWebshellEngine 构造 Webshell 检测引擎。
func NewWebshellEngine() *WebshellEngine {
	return &WebshellEngine{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// WithClient 注入自定义 HTTP 客户端。
func (e *WebshellEngine) WithClient(c *http.Client) *WebshellEngine {
	if c != nil {
		e.client = c
	}
	return e
}

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

// Run 抓取目标页面并对响应体执行 Webshell 特征检测。
func (e *WebshellEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CInsight-Scanner/0.1")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	findings := CheckContent(string(body))
	for i := range findings {
		if findings[i].URL == "" {
			findings[i].URL = target.URL
		}
	}
	return findings, nil
}

// CheckContent 对给定代码内容执行 Webshell 特征检测。
// PHP 语境门控：eval/base64_decode 等在纯前端 JS 页面属正常用法，仅在检测到
// PHP 标签时才计入，命令执行类与参数直连回调在任何语境都判高危。
func CheckContent(code string) []Finding {
	var findings []Finding
	isPHP := strings.Contains(code, "<?php") || strings.Contains(code, "<?=")
	for _, pat := range WebshellPatterns {
		if pat.phpOnly && !isPHP {
			continue
		}
		if !pat.pattern.MatchString(code) {
			continue
		}
		confidence := 0.7
		if pat.severity == SeverityCritical {
			confidence = 0.8
		}
		findings = append(findings, Finding{
			Type:        "webshell_pattern",
			Severity:    pat.severity,
			Title:       fmt.Sprintf("检测到 Webshell 特征：%s", pat.desc),
			Description: fmt.Sprintf("代码内容匹配危险模式：%s", pat.desc),
			Confidence:  confidence,
			Extra: map[string]any{
				"pattern":     pat.desc,
				"php_context": isPHP,
			},
		})
	}
	// 混淆执行链：编码/压缩解码函数回填到执行函数，且高危入口来自请求参数。
	obfuscated := isPHP && reEvalPattern.MatchString(code)
	requestDriven := reRequestVar.MatchString(code)
	if obfuscated || requestDriven {
		signals := []string{}
		if obfuscated {
			signals = append(signals, "编码函数嵌套调用")
		}
		if requestDriven {
			signals = append(signals, "请求参数直连危险函数")
		}
		findings = append(findings, Finding{
			Type:        "webshell_obfuscated",
			Severity:    SeverityCritical,
			Title:       "检测到混淆 Webshell 模式",
			Description: "代码包含 " + strings.Join(signals, "、") + " 组合，高度疑似 Webshell",
			Confidence:  0.9,
			Extra:       map[string]any{"signals": signals, "php_context": isPHP},
		})
	}
	// PHP 危险函数组合：带标签的 PHP 脚本内同时出现参数超全局变量与执行原语。
	if isPHP && reRequestVar.MatchString(code) && reDangerFunc.MatchString(strings.ToLower(code)) {
		findings = append(findings, Finding{
			Type:        "webshell_php",
			Severity:    SeverityCritical,
			Title:       "检测到 PHP Webshell",
			Description: "PHP 代码包含请求参数直连危险函数组合，疑似后门文件",
			Confidence:  0.85,
			Extra:       map[string]any{"php_context": true},
		})
	}
	return findings
}
