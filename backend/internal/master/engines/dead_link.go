package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const NameDeadLink = "dead_link"
const TypeDeadLink = "dead_link"

// DeadLinkEngine 死链监测引擎（content_security 子能力）：
// 校验页面内链接（内链+外链）健康度，4xx/5xx/连接失败/超时生成 dead_link finding。
type DeadLinkEngine struct {
	client *http.Client
	mu     sync.Mutex
	done   map[string]bool // 已检查链接去重
}

// NewDeadLinkEngine 构造死链引擎。
func NewDeadLinkEngine() *DeadLinkEngine {
	return &DeadLinkEngine{
		client: &http.Client{Timeout: 10 * time.Second},
		done:   map[string]bool{},
	}
}

// Name 返回引擎名（content_security 子能力）。
func (e *DeadLinkEngine) Name() string { return NameDeadLink }

// Enabled 策略开关：dead_link 未显式关闭即启用（fallback content_security）。
func (e *DeadLinkEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	if en, ok := p.Enabled[NameDeadLink]; ok {
		return en
	}
	en, ok := p.Enabled[NameContentSecurity]
	if !ok {
		return true
	}
	return en
}

// Check 校验一批链接，返回死链 finding。baseURL 用于解析相对链接。
func (e *DeadLinkEngine) Check(ctx context.Context, baseURL string, links []string, policy Policy) []Finding {
	var out []Finding
	timeout := 10 * time.Second
	if policy.Timeout > 0 {
		timeout = time.Duration(policy.Timeout) * time.Second
	}
	sem := make(chan struct{}, 4)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, raw := range links {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		abs := resolveURL(baseURL, raw)
		if abs == "" {
			continue
		}
		e.mu.Lock()
		if e.done[abs] {
			e.mu.Unlock()
			continue
		}
		e.done[abs] = true
		e.mu.Unlock()
		wg.Add(1)
		sem <- struct{}{}
		go func(l string) {
			defer wg.Done()
			defer func() { <-sem }()
			f, ok := e.checkOne(ctx, l, timeout)
			if ok {
				mu.Lock()
				out = append(out, f)
				mu.Unlock()
			}
		}(abs)
	}
	wg.Wait()
	return out
}

// checkOne 单链接校验，ok=true 表示死链。
func (e *DeadLinkEngine) checkOne(ctx context.Context, link string, timeout time.Duration) (Finding, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return Finding{}, false
	}
	client := *e.client
	client.Timeout = timeout
	start := time.Now()
	r, err := client.Do(req)
	if err != nil {
		return Finding{
			Type: TypeDeadLink, Severity: SeverityMedium,
			Title: "死链: 连接失败", Description: "链接 " + link + " 连接失败: " + err.Error(),
			URL: link, Confidence: 0.9,
			Extra: map[string]any{"status": "conn_error", "elapsed_ms": time.Since(start).Milliseconds()},
		}, true
	}
	defer r.Body.Close()
	ms := time.Since(start).Milliseconds()
	if r.StatusCode >= 400 {
		sev := SeverityMedium
		if r.StatusCode >= 500 {
			sev = SeverityHigh
		}
		return Finding{
			Type: TypeDeadLink, Severity: sev,
			Title: fmt.Sprintf("死链: HTTP %d", r.StatusCode),
			Description: fmt.Sprintf("链接 %s 返回 HTTP %d", link, r.StatusCode),
			URL: link, Confidence: 0.95,
			Extra: map[string]any{"status": r.StatusCode, "elapsed_ms": ms},
		}, true
	}
	// 部分服务器不支持 HEAD，降级 GET（丢弃 body）。
	return Finding{}, false
}

// resolveURL 解析相对/绝对链接，返回绝对 URL（同协议 http/https）。
func resolveURL(base, raw string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme == "" {
		u.Scheme = bu.Scheme
	}
	if u.Host == "" {
		u.Host = bu.Host
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}
