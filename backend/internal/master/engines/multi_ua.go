package engines

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	reScriptBlock = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyleBlock  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
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
	DOMFingerprint string `json:"dom_fingerprint,omitempty"` // DOM 结构指纹（SimHash）
	TextLen      int      `json:"text_len"`                  // 可见文本长度（SPA 空壳识别）
	Links        []string `json:"links,omitempty"`           // 页外链集合
	SensitiveHits []string `json:"sensitive_hits,omitempty"` // 端级敏感词命中
	SensitiveInfoHits []string `json:"sensitive_info_hits,omitempty"` // 端级敏感信息命中
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
	// 三级评分明细（基础/特征/场景）。
	BaseScore   int `json:"base_score"`
	FeatureScore int `json:"feature_score"`
	ScenarioScore int `json:"scenario_score"`
	// SimHash 端间 DOM 相似度（0~100），>90 视为一致。
	DOMSimilarity int `json:"dom_similarity"`
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

	res.BaseScore, res.FeatureScore, res.ScenarioScore, res.Score, res.Level, res.Suggestion = e.evaluate(&res)
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
	// 抓取 body 用于 DOM 指纹/文本长度/外链提取（限制 2MB 防内存）。
	body, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	bodyStr := string(body)
	out.Size = len(body)
	out.Title = extractTitle(bodyStr)
	out.DOMFingerprint = domFingerprint(bodyStr)
	visible := extractVisibleText(bodyStr)
	out.TextLen = len([]rune(visible))
	out.Links = extractPageLinks(bodyStr, target)
	// 端级内容安全监测：敏感词 / 敏感信息命中计入评分。
	out.SensitiveHits = matchSensitiveWords(target, visible)
	out.SensitiveInfoHits = matchSensitiveInfo(target, visible)
	return out
}

// extractVisibleText 提取可见文本（去除 script/style/标签）。
func extractVisibleText(html string) string {
	noScript := reScriptBlock.ReplaceAllString(html, " ")
	noStyle := reStyleBlock.ReplaceAllString(noScript, " ")
	text := regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(noStyle, " ")
	return strings.TrimSpace(text)
}

// matchSensitiveWords 端级敏感词命中（复用 SensitiveWordEngine 词库）。
func matchSensitiveWords(source, text string) []string {
	sw := NewSensitiveWordEngine()
	findings := sw.Match(source, "", text, nil)
	var out []string
	for _, f := range findings {
		if w, ok := f.Extra["word"].(string); ok && w != "" {
			out = append(out, w)
		}
	}
	return out
}

// matchSensitiveInfo 端级敏感信息命中（复用 SensitiveInfoEngine 规则集）。
func matchSensitiveInfo(source, text string) []string {
	si := NewSensitiveInfoEngine()
	_, hits := si.Match(source, map[string]string{"body": text})
	var out []string
	for _, h := range hits {
		out = append(out, h.Group+":"+h.Name)
	}
	return out
}

// extractTitle 提取 <title> 内容。
func extractTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>([^<]*)</title>`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// domFingerprint 计算 DOM 结构 SimHash 指纹：标签序列 → 64bit 哈希。
func domFingerprint(html string) string {
	reTag := regexp.MustCompile(`(?is)<\/?([a-zA-Z][a-zA-Z0-9]*)`)
	tags := reTag.FindAllStringSubmatch(html, -1)
	// 取前 200 个标签作为指纹输入。
	tagSeq := make([]string, 0, len(tags))
	for i, m := range tags {
		if i >= 200 {
			break
		}
		if len(m) > 1 {
			tagSeq = append(tagSeq, m[1])
		}
	}
	if len(tagSeq) == 0 {
		return ""
	}
	// SimHash：每标签 hash 贡献位向量加权。
	v := make([]int, 64)
	for _, tag := range tagSeq {
		h := fnv.New64a()
		h.Write([]byte(tag))
		hv := h.Sum64()
		for i := 0; i < 64; i++ {
			bit := (hv >> uint(i)) & 1
			if bit == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}
	var fp uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fp |= 1 << uint(i)
		}
	}
	return fmt.Sprintf("%016x", fp)
}

// simHashDistance 两 SimHash 十六进制串的汉明距离。
func simHashDistance(a, b string) int {
	if a == "" || b == "" {
		return -1
	}
	var ai, bi uint64
	fmt.Sscanf(a, "%016x", &ai)
	fmt.Sscanf(b, "%016x", &bi)
	return popcount(ai ^ bi)
}

func popcount(x uint64) int {
	c := 0
	for x != 0 {
		x &= x - 1
		c++
	}
	return c
}

// extractPageLinks 提取页面出站链接（外部域），用于各端外链集合对比。
func extractPageLinks(html, base string) []string {
	re := regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["']`)
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		ref, err := url.Parse(strings.TrimSpace(m[1]))
		if err != nil {
			continue
		}
		resolved := baseURL.ResolveReference(ref)
		if !resolved.IsAbs() || resolved.Host == "" {
			continue
		}
		if strings.EqualFold(resolved.Host, baseURL.Host) {
			continue
		}
		if seen[resolved.String()] {
			continue
		}
		seen[resolved.String()] = true
		out = append(out, resolved.String())
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

// evaluate 输出三级加权综合评估：
// 基础分（可用性/状态码一致性）+ 特征分（SPA 空壳/DOM 结构异常/独有外链）+ 场景分（端差异化宕机/定向投毒）。
// probe_failed 单端失败降权（不整体判宕），SimHash>90% 视为 DOM 一致不加分。
func (e *MultiUAAssessor) evaluate(res *MultiUAResult) (baseScore, featureScore, scenarioScore, score int, level, suggestion string) {
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
	// SimHash DOM 相似度：取前两个成功探针比对。
	res.DOMSimilarity = 100
	if len(okProbes) >= 2 {
		d := simHashDistance(okProbes[0].DOMFingerprint, okProbes[1].DOMFingerprint)
		if d >= 0 {
			res.DOMSimilarity = 100 - d*100/64
		}
	}
	// SPA 空壳识别：成功探针文本极短（<20 字符）但存在 DOM 结构（JS 渲染壳）。
	spaLike := 0
	for _, pr := range okProbes {
		if pr.TextLen < 20 && pr.DOMFingerprint != "" {
			spaLike++
		}
	}
	if spaLike >= len(okProbes)/2 && len(okProbes) > 0 {
		res.SPASuspected = true
	}

	// 三级评分（风险分，0=无风险）。
	baseScore = 0
	// 基础分：全端失败 100；部分端失败降权（每端 +20，上限 60）。
	if len(failed) == len(res.Probes) && len(res.Probes) > 0 {
		baseScore = 100
		res.EndDown = []string{"all"}
	} else {
		baseScore = 20 * len(failed)
		if baseScore > 60 {
			baseScore = 60
		}
	}
	// 特征分：SPA 空壳 +15（待复核），DOM 相似度低 +10（结构差异），端级敏感词/敏感信息命中 +25/端。
	featureScore = 0
	if res.SPASuspected {
		featureScore += 15
	}
	if res.DOMSimilarity < 90 {
		featureScore += 10
	}
	for _, pr := range okProbes {
		if len(pr.SensitiveHits) > 0 || len(pr.SensitiveInfoHits) > 0 {
			featureScore += 25
		}
	}
	// 场景分：端差异化宕机 +30/端，移动端定向投毒（mobile/wechat/mobile_viewport 失败）加重。
	scenarioScore = 0
	for _, dn := range res.EndDown {
		scenarioScore += 30
		if dn == "mobile" || dn == "wechat" || dn == "mobile_viewport" {
			scenarioScore += 10 // 移动端定向投毒加重
		}
	}
	scenarioScore += 15 * len(res.EndDiff)

	score = baseScore + featureScore + scenarioScore
	if score > 100 {
		score = 100
	}
	switch {
	case score >= 85:
		level, suggestion = "严重", "多端差异化宕机或移动端定向投毒，立即排查端侧拦截/降级并处置"
	case score >= 60:
		level, suggestion = "高危", "端间可用性不一致，生成告警并排查"
	case score >= 30:
		level, suggestion = "可疑", "端间存在差异或 SPA 结构异常，待人工复核"
	default:
		level, suggestion = "正常", "各端一致，仅记录"
	}
	return baseScore, featureScore, scenarioScore, score, level, suggestion
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
