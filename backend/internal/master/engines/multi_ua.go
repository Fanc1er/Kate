package engines

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	// NameMultiUA MultiUAAssessor 引擎名。
	NameMultiUA = "multi_ua"
	// TypeMultiUAAvailability 端差异化宕机 finding 类型。
	TypeMultiUAAvailability = "multi_ua_availability"
)

// UAProbe 一种 UA 探针配置。
type UAProbe struct {
	Name string // pc / mobile / wechat / mobile_viewport
	UA   string
	Width, Height int
}

// ProbeResult 单探针抓取结果。
type ProbeResult struct {
	Name         string   `json:"name"`
	Status       int      `json:"status"`
	StatusCode   int      `json:"status_code"`
	LatencyMS    int64    `json:"latency_ms"`
	Redirects    []string `json:"redirects"`
	RedirectChain int     `json:"redirect_count"`
	Err          string   `json:"error,omitempty"`
	Failed       bool     `json:"failed"`
	FinalURL     string   `json:"final_url"`
	Title        string   `json:"title"`
	Size         int      `json:"size"`
}

// MultiUAResult 综合评估结果。
type MultiUAResult struct {
	Probes       []ProbeResult `json:"probes"`
	Score        int           `json:"score"`
	Level        string        `json:"level"`
	Suggestion   string        `json:"suggestion"`
	EndDiff      []string      `json:"end_diff,omitempty"`
	EndDown      []string      `json:"end_down,omitempty"`
	SPASuspected bool          `json:"spa_suspected"`
}

// MultiUAAssessor 多端 UA 综合评估器：四探针抓取 + 端间可用性一致性判定。
type MultiUAAssessor struct {
	client *http.Client
	Probes []UAProbe
	// SlowThresholdMS 延迟一致性阈值（默认 3000ms）。
	SlowThresholdMS int
}

// NewMultiUAAssessor 构造评估器（默认四探针）。
func NewMultiUAAssessor() *MultiUAAssessor {
	return &MultiUAAssessor{
		client: &http.Client{},
		Probes: []UAProbe{
			{Name: "pc", UA: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", Width: 1440, Height: 900},
			{Name: "mobile", UA: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", Width: 375, Height: 812},
			{Name: "wechat", UA: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.49(0x18003123) NetType/WIFI Language/zh_CN", Width: 375, Height: 812},
			{Name: "mobile_viewport", UA: "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36", Width: 375, Height: 812},
		},
	}
}

// Name 返回引擎名。
func (e *MultiUAAssessor) Name() string { return NameMultiUA }

// Enabled 策略开关：multi_ua 未显式关闭即启用。
func (e *MultiUAAssessor) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameMultiUA]
	if !ok {
		return true
	}
	return en
}

// Run 执行多端评估，返回端差异化宕机/不一致 finding。
func (e *MultiUAAssessor) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	res := e.Assess(ctx, target.URL, time.Duration(p.Timeout)*time.Second)
	var findings []Finding
	if len(res.EndDown) > 0 {
		findings = append(findings, Finding{
			Type: TypeMultiUAAvailability, Severity: SeverityHigh,
			Title:       "端差异化宕机: " + joinProbes(res.EndDown),
			Description: fmt.Sprintf("部分探针可用性异常（%s），其余端正常，疑似端差异化宕机/移动端拦截", joinProbes(res.EndDown)),
			URL:         target.URL, Confidence: 0.85,
			Extra: map[string]any{"multi_ua": res, "end_down": res.EndDown, "end_diff": res.EndDiff},
		})
	}
	if len(res.EndDiff) > 0 && len(res.EndDown) == 0 {
		findings = append(findings, Finding{
			Type: TypeMultiUAAvailability, Severity: SeverityMedium,
			Title:       "端间状态码/延迟不一致",
			Description: fmt.Sprintf("各探针状态码或响应时间存在差异（%s）", joinProbes(res.EndDiff)),
			URL:         target.URL, Confidence: 0.7,
			Extra: map[string]any{"multi_ua": res, "end_diff": res.EndDiff},
		})
	}
	return findings, nil
}

// Assess 执行多探针抓取并输出综合评估。
func (e *MultiUAAssessor) Assess(ctx context.Context, target string, timeout time.Duration) MultiUAResult {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	res := MultiUAResult{Probes: make([]ProbeResult, 0, len(e.Probes))}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, pr := range e.Probes {
		wg.Add(1)
		go func(p UAProbe) {
			defer wg.Done()
			r := e.probe(ctx, target, p, timeout)
			mu.Lock()
			res.Probes = append(res.Probes, r)
			mu.Unlock()
		}(pr)
	}
	wg.Wait()

	res.Score, res.Level, res.Suggestion = e.evaluate(&res)
	return res
}

// probe 单探针抓取：状态码、延迟、重定向链路。
func (e *MultiUAAssessor) probe(ctx context.Context, target string, p UAProbe, timeout time.Duration) ProbeResult {
	out := ProbeResult{Name: p.Name}
	client := *e.client
	client.Timeout = timeout
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		if len(via) > 0 {
			out.Redirects = append(out.Redirects, req.URL.String())
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		out.Failed = true
		out.Err = err.Error()
		return out
	}
	req.Header.Set("User-Agent", p.UA)
	if p.Width > 0 {
		req.Header.Set("Viewport-Width", fmt.Sprint(p.Width))
	}
	start := time.Now()
	r, err := client.Do(req)
	out.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		out.Failed = true
		out.Err = err.Error()
		return out
	}
	defer r.Body.Close()
	out.Status = r.StatusCode
	out.StatusCode = r.StatusCode
	out.RedirectChain = len(out.Redirects)
	out.FinalURL = r.Request.URL.String()
	return out
}

// evaluate 输出综合评估：检测端间状态码/延迟不一致与端差异化宕机。
func (e *MultiUAAssessor) evaluate(res *MultiUAResult) (score int, level, suggestion string) {
	var okProbes []ProbeResult
	var failed []string
	for _, pr := range res.Probes {
		// 连接失败或 4xx/5xx 错误码视为不可用端。
		if pr.Failed || pr.StatusCode == 0 || pr.StatusCode >= 400 {
			failed = append(failed, pr.Name)
		} else {
			okProbes = append(okProbes, pr)
		}
	}
	// 端差异化宕机：部分探针不可用，其余正常；全端不可用视为整体宕机。
	if len(failed) > 0 {
		res.EndDown = append(res.EndDown, failed...)
	}
	// 状态码一致性。
	statuses := map[int]int{}
	for _, pr := range okProbes {
		statuses[pr.StatusCode]++
	}
	if len(statuses) > 1 {
		res.EndDiff = append(res.EndDiff, "状态码不一致")
	}
	// 延迟一致性（最大延迟超慢阈值且与最小延迟差 > 3 倍视为异常）。
	if len(okProbes) > 1 {
		maxMS, minMS := int64(0), int64(1<<62)
		for _, pr := range okProbes {
			if pr.LatencyMS > maxMS {
				maxMS = pr.LatencyMS
			}
			if pr.LatencyMS < minMS {
				minMS = pr.LatencyMS
			}
		}
		slow := int64(e.SlowThresholdMS)
		if slow <= 0 {
			slow = 3000
		}
		if maxMS >= slow && minMS > 0 && maxMS > minMS*3 {
			res.EndDiff = append(res.EndDiff, "延迟差异过大")
		}
	}
	// 计分：每端差异 +15，宕机端 +30（多端宕机再加重），SPA 降权。
	score = 0
	if len(res.EndDown) > 0 {
		score += 30 * len(res.EndDown)
	}
	score += 15 * len(res.EndDiff)
	if score > 100 {
		score = 100
	}
	switch {
	case score >= 85:
		level, suggestion = "严重", "多端差异化宕机，立即排查端侧拦截/降级并处置"
	case score >= 60:
		level, suggestion = "高危", "端间可用性不一致，生成告警并排查"
	case score >= 30:
		level, suggestion = "可疑", "端间存在差异，待人工复核"
	default:
		level, suggestion = "正常", "各端一致，仅记录"
	}
	return score, level, suggestion
}

func joinProbes(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ","
		}
		out += n
	}
	return out
}
