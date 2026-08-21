package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const NameAvailability = "availability"

// AvailabilityEngine 可用性监测引擎：HTTP 探针 + 连续失败判定 + 访问速度阈值。
type AvailabilityEngine struct {
	client *http.Client
}

// NewAvailabilityEngine 构造可用性引擎。
func NewAvailabilityEngine() *AvailabilityEngine {
	return &AvailabilityEngine{client: &http.Client{}}
}

// Name 返回引擎名。
func (e *AvailabilityEngine) Name() string { return NameAvailability }

// Enabled 策略开关：availability 未显式关闭即启用。
func (e *AvailabilityEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameAvailability]
	if !ok {
		return true
	}
	return en
}

// Run 执行可用性探测：连续 fail_count 次失败判定不可用（指数间隔重试），
// 响应耗时超过 slow_threshold_ms 判定访问速度异常。
func (e *AvailabilityEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	failCount := p.FailCount
	if failCount <= 0 {
		failCount = 3
	}
	slowThreshold := p.SlowThresholdMS
	if slowThreshold <= 0 {
		slowThreshold = 3000
	}
	var findings []Finding
	var lastErr error
	var resp *http.Response
	var ms int64
	for attempt := 0; attempt < failCount; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
		if err != nil {
			lastErr = err
			break
		}
		start := time.Now()
		r, err := e.client.Do(req)
		ms = time.Since(start).Milliseconds()
		if err != nil {
			lastErr = err
			if attempt < failCount-1 {
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			continue
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		resp = r
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
		_ = r.Body.Close()
		break
	}
	if lastErr != nil {
		findings = append(findings, Finding{
			Type: "unreachable", Severity: SeverityHigh,
			Title: "资产不可达", Description: "连续探测失败: " + lastErr.Error(),
			URL: target.URL, Confidence: 0.95,
		})
		return findings, nil
	}
	if resp == nil {
		return findings, nil
	}
	if resp.StatusCode >= 400 {
		findings = append(findings, Finding{
			Type: "http_error", Severity: severityForStatus(resp.StatusCode),
			Title: fmt.Sprintf("HTTP %d 状态异常", resp.StatusCode),
			Description: fmt.Sprintf("资产返回 HTTP %d，期望 2xx/3xx", resp.StatusCode),
			URL: target.URL, Confidence: 0.9,
		})
	}
	if ms >= int64(slowThreshold) {
		findings = append(findings, Finding{
			Type: "slow_response", Severity: SeverityMedium,
			Title: "响应速度异常", Description: fmt.Sprintf("响应耗时 %dms 超过阈值 %dms", ms, slowThreshold),
			URL: target.URL, Confidence: 0.8,
		})
	}
	return findings, nil
}

func severityForStatus(code int) string {
	switch {
	case code >= 500:
		return SeverityHigh
	case code >= 400:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// defaultPolicy 构建默认策略（worker 兼容）。
func DefaultAvailabilityPolicy() Policy {
	return Policy{Enabled: map[string]bool{NameAvailability: true}, FailCount: 3, SlowThresholdMS: 3000}
}
