// Package main 是 CInsight 平台的 Worker 进程入口。
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Fanc1er/Kate/backend/pkg/config"
	"github.com/Fanc1er/Kate/backend/internal/master/engines"
)

func main() {
	cfg := config.Load()
	base := "http://localhost:" + cfg.Port
	if v := os.Getenv("CINSIGHT_MASTER_URL"); v != "" {
		base = v
	}
	w := &worker{base: base, pollMS: cfg.WorkerPollMS, heartbeatMS: cfg.WorkerHeartbeat, screenshots: NewScreenshotter(2, detectChromium())}
	ctx, cancel := context.WithCancel(context.Background())

	// 从环境变量读取已注册凭证；未注册时尝试 Bootstrap。
	if os.Getenv("CINSIGHT_WORKER_CLIENT_ID") == "" {
		if err := w.register(cfg); err != nil {
			log.Fatalf("worker register: %v", err)
		}
	} else {
		w.clientID = os.Getenv("CINSIGHT_WORKER_CLIENT_ID")
		w.clientSecret = os.Getenv("CINSIGHT_WORKER_CLIENT_SECRET")
		w.orgID = atoi(os.Getenv("CINSIGHT_WORKER_ORG_ID"))
	}

	go w.heartbeatLoop(ctx)
	go w.pollLoop(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("shutting down worker...")
	cancel()
	w.outbox.flush()
}

// worker 拉取任务 → 执行 → 回传结果，断网时结果写入 Outbox。
type worker struct {
	base         string
	pollMS       int
	heartbeatMS  int
	clientID     string
	clientSecret string
	orgID        int64
	version      string
	mu           sync.Mutex
	outbox       outbox
	screenshots  *Screenshotter
}

// register 使用 Bootstrap Token 注册并保存长期凭证。
func (w *worker) register(cfg *config.Config) error {
	token := os.Getenv("CINSIGHT_WORKER_BOOT_TOKEN")
	if token == "" {
		return fmt.Errorf("CINSIGHT_WORKER_BOOT_TOKEN 未设置")
	}
	body, _ := json.Marshal(map[string]string{
		"token": token, "name": hostname(), "version": "0.1.0", "ip": "127.0.0.1",
	})
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			OrgID        int64  `json:"org_id"`
		} `json:"data"`
	}
	if err := w.post("/api/v1/worker/register", body, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("register failed: %s", resp.Message)
	}
	w.clientID = resp.Data.ClientID
	w.clientSecret = resp.Data.ClientSecret
	w.orgID = resp.Data.OrgID
	w.version = "0.1.0"
	log.Printf("worker registered: client_id=%s org=%d", w.clientID, w.orgID)
	return nil
}

// heartbeatLoop 心跳上报。
func (w *worker) heartbeatLoop(ctx context.Context) {
	interval := time.Duration(w.heartbeatMS) * time.Millisecond
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			body, _ := json.Marshal(map[string]any{"load": 0.1, "version": w.version})
			var resp struct {
				Code int `json:"code"`
			}
			if err := w.post("/api/v1/worker/heartbeat", body, &resp); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

// pollLoop 拉取任务并执行。
func (w *worker) pollLoop(ctx context.Context) {
	interval := time.Duration(w.pollMS) * time.Millisecond
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pullAndRun()
		}
	}
}

// pullAndRun 拉取一个任务，执行并回传。
func (w *worker) pullAndRun() {
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Task           *taskPayload  `json:"task"`
			Policy         *policyPayload `json:"policy"`
			Asset          *assetPayload `json:"asset"`
			EngineSwitches map[string]any   `json:"engine_switches"`
			Engines        []string        `json:"engines"`
			KeywordRules   []keywordRule   `json:"keyword_rules"`
			DomainRules    []domainRule    `json:"domain_rules"`
			Recursion      map[string]any  `json:"recursion"`
		} `json:"data"`
	}
	if err := w.post("/api/v1/worker/pull", []byte("{}"), &resp); err != nil {
		log.Printf("pull failed: %v", err)
		return
	}
	if resp.Code != 0 || resp.Data.Task == nil {
		return
	}
	t := resp.Data.Task
	if resp.Data.Policy == nil {
		resp.Data.Policy = &policyPayload{}
	}
	resp.Data.Policy.Engines = resp.Data.Engines
	resp.Data.Policy.KeywordRules = resp.Data.KeywordRules
	resp.Data.Policy.DomainRules = resp.Data.DomainRules
	if avail, ok := resp.Data.EngineSwitches["availability"].(map[string]any); ok {
		if fc, ok := avail["fail_count"].(float64); ok && fc > 0 {
			resp.Data.Policy.FailCount = int(fc)
		}
		if st, ok := avail["slow_threshold_ms"].(float64); ok && st > 0 {
			resp.Data.Policy.SlowThresholdMS = int(st)
		}
	}
	if ev, ok := resp.Data.EngineSwitches["evidence"].(map[string]any); ok {
		if en, ok := ev["enabled"].(bool); ok {
			resp.Data.Policy.EnableEvidence = en
		}
	}
	// 递归扫描配置。
	if rc, ok := resp.Data.Recursion["scan_depth"].(float64); ok {
		resp.Data.Policy.ScanDepth = int(rc)
	}
	if rc, ok := resp.Data.Recursion["concurrency_limit"].(float64); ok {
		resp.Data.Policy.ConcurrencyLimit = int(rc)
	}
	if v, ok := resp.Data.Recursion["allow_static"].(bool); ok {
		resp.Data.Policy.AllowStatic = v
	}
	if v, ok := resp.Data.Recursion["same_origin"].(bool); ok {
		resp.Data.Policy.SameOrigin = v
	}
	if v, ok := resp.Data.Recursion["crawl_subpages"].(bool); ok {
		resp.Data.Policy.CrawlSubpages = v
	}
	log.Printf("task %d received, executing asset %d...", t.ID, resp.Data.Asset.ID)
	result := w.execute(context.Background(), t, resp.Data.Policy, resp.Data.Asset)
	w.report(t.ID, result)
}

// report 回传结果，失败写入 Outbox。
func (w *worker) report(taskID int64, result *resultPayload) {
	body, err := json.Marshal(result)
	if err != nil {
		return
	}
	var resp struct {
		Code int `json:"code"`
	}
	if err := w.post("/api/v1/worker/result", body, &resp); err != nil || resp.Code != 0 {
		log.Printf("report failed (outbox): %v code=%d", err, resp.Code)
		w.outbox.push(w, body)
	}
}

// execute 执行扫描（MVP：可用性探针 + 内容安全引擎骨架）。
func (w *worker) execute(ctx context.Context, t *taskPayload, p *policyPayload, a *assetPayload) *resultPayload {
	now := time.Now()
	result := &resultPayload{
		ResultID:  fmt.Sprintf("%d-%d", t.ID, now.UnixNano()),
		TaskID:    t.ID,
		Status:    "completed",
		Progress:  100,
		Findings:  []findingPayload{},
		Metrics:   map[string]any{"started_at": now.Format(time.RFC3339)},
	}
	// 任务级执行超时：策略 timeout 分钟上限，Worker 侧超时中止并标记 task_timeout=true。
	timeout := 60 * time.Minute
	if p != nil && p.Timeout > 0 {
		timeout = time.Duration(p.Timeout) * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 可用性引擎配置：fail_count（连续失败判定次数）+ slow_threshold_ms（访问速度阈值）。
	failCount := 3
	slowThreshold := 3000
	if p != nil {
		if p.FailCount > 0 {
			failCount = p.FailCount
		}
		if p.SlowThresholdMS > 0 {
			slowThreshold = p.SlowThresholdMS
		}
	}

	// 断点续扫：任务重启/重拉时已抓取的 URL 直接跳过，标记 resumed 并返回完成。
	if w.alreadyCrawled(t.ID, a.URL) {
		result.Message = "断点续扫：该 URL 已抓取，跳过重复执行"
		result.Metrics["resumed"] = true
		return result
	}

	// 引擎开关：availability 未启用则跳过探测直接返回完成（其余引擎见 engines 列表）。
	if !p.engineEnabled("availability") {
		result.Message = "availability 引擎未启用，跳过探测"
		result.Metrics["engines_skipped"] = []string{"availability"}
		return result
	}
	result.Metrics["engines"] = p.Engines

	// 可用性探针：连续 fail_count 次失败才判定不可用（指数间隔重试）。
	// 成功后保留响应：用于生成 Req/Resp/HTML 快照证据与 HAR。
	var lastErr error
	var resp *http.Response
	var respBody []byte
	var respHeaders http.Header
	var ms int64
	client := &http.Client{}
	for attempt := 0; attempt < failCount; attempt++ {
		if ctx.Err() != nil {
			result.Status = "failed"
			result.TaskTimeout = true
			result.Message = "任务执行超时"
			return result
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		start := time.Now()
		r, err := client.Do(req)
		ms = time.Since(start).Milliseconds()
		if err != nil {
			lastErr = err
			if attempt < failCount-1 {
				select {
				case <-time.After(time.Duration(attempt+1) * time.Second):
				case <-ctx.Done():
					result.Status = "failed"
					result.TaskTimeout = true
					result.Message = "任务执行超时"
					return result
				}
			}
			continue
		}
		if r != nil {
			if resp != nil {
				r.Body.Close()
			}
			resp = r
			body, readErr := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body.Close()
			if readErr == nil {
				respBody = body
			}
			respHeaders = r.Header
		}
		break
	}
	if lastErr != nil {
		result.Status = "failed"
		result.Message = "可用性探测失败: " + lastErr.Error()
		result.Metrics["latency_ms"] = ms
		return result
	}

	// 可用性判定。
	if resp != nil && resp.StatusCode >= 400 {
		result.Findings = append(result.Findings, findingPayload{
			EngineName: "availability", Type: "http_error", Severity: severityForStatus(resp.StatusCode),
			Title: fmt.Sprintf("HTTP %d 状态异常", resp.StatusCode), Description: fmt.Sprintf("资产返回 HTTP %d，期望 2xx/3xx", resp.StatusCode),
			URL: a.URL, Confidence: 0.9,
		})
	}
	if ms >= int64(slowThreshold) {
		result.Findings = append(result.Findings, findingPayload{
			EngineName: "availability", Type: "slow_response", Severity: "medium",
			Title: "响应速度异常", Description: fmt.Sprintf("响应耗时 %dms 超过阈值 %dms", ms, slowThreshold),
			URL: a.URL, Confidence: 0.8,
		})
	}
	if resp != nil {
		result.Metrics["status_code"] = resp.StatusCode
	}
	result.Metrics["latency_ms"] = ms
	result.Metrics["fail_count"] = failCount

	// 多端 UA 综合评估：四探针抓取比对端间状态码/延迟，标记端差异化宕机。
	if p != nil && p.engineEnabled("multi_ua") {
		assessor := engines.NewMultiUAAssessor()
		assessor.SlowThresholdMS = slowThreshold
		uaRes := assessor.Assess(ctx, a.URL, 30*time.Second)
		// 评估详情写入 finding Extra（GET /api/v1/findings/:id 展示 extra.multi_ua）。
		uaExtra := map[string]any{
			"probes": uaRes.Probes, "score": uaRes.Score,
			"base_score": uaRes.BaseScore, "feature_score": uaRes.FeatureScore, "scenario_score": uaRes.ScenarioScore,
			"level": uaRes.Level, "suggestion": uaRes.Suggestion,
			"end_down": uaRes.EndDown, "end_diff": uaRes.EndDiff,
			"spa_suspected": uaRes.SPASuspected, "dom_similarity": uaRes.DOMSimilarity,
		}
		if len(uaRes.EndDown) > 0 || len(uaRes.EndDiff) > 0 {
			// 异常：生成 finding。
			sev := engines.SeverityMedium
			title := "端间状态码/延迟不一致"
			desc := fmt.Sprintf("各探针状态码或响应时间存在差异（%s）", strings.Join(uaRes.EndDiff, ","))
			if len(uaRes.EndDown) > 0 {
				sev = engines.SeverityHigh
				title = "端差异化宕机: " + strings.Join(uaRes.EndDown, ",")
				desc = fmt.Sprintf("部分探针可用性异常（%s），其余端正常，疑似端差异化宕机/移动端拦截",
					strings.Join(uaRes.EndDown, ","))
			}
			result.Findings = append(result.Findings, findingPayload{
				EngineName: "multi_ua", Type: "multi_ua_availability", Severity: sev,
				Title: title, Description: desc,
				URL: a.URL, Confidence: 0.85,
				Extra: map[string]any{"multi_ua": uaExtra},
			})
		} else {
			// 各端一致：info 记录评估报告（含评分/SPA/DOM 相似度），供前端展示。
			extra := map[string]any{"multi_ua": uaExtra}
			if uaRes.SPASuspected {
				extra["spa_suspected"] = true
			}
			result.Findings = append(result.Findings, findingPayload{
				EngineName: "multi_ua", Type: "multi_ua_evaluation", Severity: engines.SeverityInfo,
				Title: "多端 UA 综合评估", Description: "各端一致，评估分 " + fmt.Sprint(uaRes.Score) + "（" + uaRes.Level + "）",
				URL: a.URL, Confidence: 0.95, Extra: extra,
			})
		}
		result.Metrics["multi_ua"] = map[string]any{
			"score": uaRes.Score, "level": uaRes.Level, "end_down": uaRes.EndDown, "end_diff": uaRes.EndDiff,
		}
	}

	// 内容安全子能力：敏感词监测 + 敏感信息 + 死链监测（在可用性探测成功的响应 HTML 上执行）。
	if resp != nil && p != nil && len(respBody) > 0 {
		contentFindings := runContentEngines(ctx, a.URL, respBody, p, respHeaders)
		result.Findings = append(result.Findings, contentFindings...)
		if len(contentFindings) > 0 {
			result.Metrics["content_findings"] = len(contentFindings)
		}
	}

	// 递归扫描与资产发现：按策略深度抓取子页面，解析静态资源/子域名/接口路径写 assets，
	// 子页面执行内容子能力（crawl_subpages）。crawl_subpages=false 时深度强制 1 仅测种子页。
	if resp != nil && p != nil {
		assets, crawled, subFindings, _ := w.recursiveCrawl(ctx, a.URL, p, respHeaders)
		if len(assets) > 0 {
			result.Discovered = assets
			result.Metrics["discovered_assets"] = len(assets)
		}
		if crawled > 0 {
			result.Metrics["crawled_pages"] = crawled
			result.Metrics["recursion_depth"] = p.ScanDepth
		}
		if len(subFindings) > 0 {
			result.Findings = append(result.Findings, subFindings...)
		}
	}

	// 证据链生成：Req/Resp 快照 + HTML 快照 + HAR（均内联回传，Master 落库校验）。
	if resp != nil && p != nil && p.EnableEvidence {
		evs := buildEvidence(a.URL, resp.StatusCode, respHeaders, respBody, ms)
		// 页面渲染截图采集：chromium 无头渲染 + DOMContentLoaded+2s + PNG。
		// 受 screenshot 引擎开关控制（未列入时默认启用）；超时/渲染失败/并发池繁忙降级
		// screenshot:skipped（不阻断主流程）。
		if w.screenshots != nil && resp.StatusCode < 400 && p.engineEnabled("screenshot") {
			png, shotErr := w.screenshots.Capture(ctx, a.URL)
			if shotErr == nil && len(png) > 0 {
				evs = append(evs, inlineEvidence{
					Kind: "screenshot", Content: base64Encode(png), SHA256: sha256Hex(png),
				})
			} else {
				result.Metrics["screenshot"] = "skipped"
				if shotErr != nil {
					log.Printf("screenshot skipped: %v", shotErr)
				}
			}
		}
		result.Evidence = evs
		for i := range result.Findings {
			result.Findings[i].InlineEvidence = evs
		}
		// 行号定位：HTML 快照中定位首个匹配引擎关键词的行（如 http_error / slow_response 文案）。
		for i := range result.Findings {
			if f := &result.Findings[i]; len(respBody) > 0 && f.LineNo == 0 {
				f.LineNo = locateLine(respBody, f.Type)
			}
		}
		// ≥1MB 证据改走分片上传（upload_id 断点续传），替换为 evidence_id 引用。
		chunkedIDs := []int64{}
		kept := result.Evidence[:0]
		for _, ev := range result.Evidence {
			if len(ev.Content) > 1<<20 { // base64 长度超 1MB
				if id, ok := w.uploadEvidenceChunked(ev); ok {
					chunkedIDs = append(chunkedIDs, id)
					continue
				}
			}
			kept = append(kept, ev)
		}
		result.Evidence = kept
		if len(chunkedIDs) > 0 {
			for j := range result.Findings {
				result.Findings[j].EvidenceIDs = append(result.Findings[j].EvidenceIDs, chunkedIDs...)
			}
		}
	}

	// 断点续扫：记录本任务已抓取的 URL（local:crawled:{task_id}）。
	_ = w.recordCrawled(t.ID, a.URL)
	return result
}

// buildEvidence 组装 Req/Resp/HTML/HAR 证据链（内联 base64，<1MB 上限截断）。
func buildEvidence(url string, status int, hdr http.Header, body []byte, ms int64) []inlineEvidence {
	out := make([]inlineEvidence, 0, 4)
	now := time.Now()
	// Req 快照。
	reqText := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nAccept: */*\r\n\r\n", url, hostOf(url))
	out = append(out, inlineEvidence{Kind: "req", Content: base64Encode([]byte(reqText)), SHA256: sha256Hex([]byte(reqText))})
	// Resp 快照（状态行 + 响应头 + body 摘要）。
	respText := fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status))
	for k, vs := range hdr {
		for _, v := range vs {
			respText += k + ": " + v + "\r\n"
		}
	}
	respText += "\r\n"
	respHead := []byte(respText)
	out = append(out, inlineEvidence{Kind: "resp", Content: base64Encode(respHead), SHA256: sha256Hex(respHead)})
	// HTML 快照（含正文，限 256KB）。
	if len(body) > 0 {
		html := body
		if len(html) > 256<<10 {
			html = html[:256<<10]
		}
		out = append(out, inlineEvidence{Kind: "html", Content: base64Encode(html), SHA256: sha256Hex(html)})
	}
	// HAR 1.2（gzip 压缩后内联）。
	if har := buildHAR(url, status, hdr, body, ms, now); har != nil {
		out = append(out, inlineEvidence{Kind: "har", Content: base64Encode(har), SHA256: sha256Hex(har)})
	}
	return out
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Host
	}
	return raw
}

// locateLine 在 HTML 源码中定位首个匹配关键词的行号（1 起）。
func locateLine(body []byte, keyword string) int {
	lines := bytes.Split(body, []byte("\n"))
	for i, line := range lines {
		if bytes.Contains(line, []byte(keyword)) {
			return i + 1
		}
	}
	return 0
}

// runContentEngines 执行内容安全子能力（敏感词 + 敏感信息 + 死链），按 engine_switches 控制。
func runContentEngines(ctx context.Context, pageURL string, body []byte, p *policyPayload, hdr http.Header) []findingPayload {
	var out []findingPayload
	if p == nil {
		return out
	}
	html := string(body)

	// 敏感词监测（sensitive_word）。
	if p.engineEnabled("sensitive_word") {
		title, text := extractHTMLText(html)
		engine := engines.NewSensitiveWordEngine()
		for _, f := range engine.Match(pageURL, title, text, nil) {
			out = append(out, findingPayload{
				EngineName: "content_security", Type: f.Type, Severity: f.Severity,
				Title: f.Title, Description: f.Description, URL: f.URL,
				Confidence: f.Confidence,
			})
		}
	}

	// AI 文本分类（content_violation）：配置了 LLM 时对正文分类，未配置静默。
	if p.engineEnabled("ai_classify") {
		out = append(out, runAIClassify(ctx, pageURL, body)...)
	}

	// 敏感信息监测（sensitive_info）：按 scope 分层匹配 request line/header/body。
	if p.engineEnabled("sensitive_info") {
		eng := engines.NewSensitiveInfoEngine()
		samples := map[string]string{
			"request line":    "GET " + pageURL + " HTTP/1.1",
			"request header":  "Authorization: " + hdr.Get("Authorization") + "\n" + "Cookie: " + hdr.Get("Cookie"),
			"response header": formatHeaders(hdr),
			"response body":   html,
			"body":            html,
		}
		fs, hits := eng.Match(pageURL, samples)
		for _, f := range fs {
			out = append(out, findingPayload{
				EngineName: "content_security", Type: f.Type, Severity: f.Severity,
				Title: f.Title, Description: f.Description, URL: f.URL,
				Confidence: f.Confidence,
				Extra:      map[string]any{"sensitive_info_hits": hits},
			})
		}
	}

	// 死链监测（dead_link）：解析页面链接逐一校验健康度。
	if p.engineEnabled("dead_link") {
		links := extractLinks(html)
		engine := engines.NewDeadLinkEngine()
		pol := engines.Policy{Timeout: p.Timeout}
		for _, f := range engine.Check(ctx, pageURL, links, pol) {
			out = append(out, findingPayload{
				EngineName: "content_security", Type: f.Type, Severity: f.Severity,
				Title: f.Title, Description: f.Description, URL: f.URL,
				Confidence: f.Confidence,
			})
		}
	}

	// 关键词监测（keyword）：rule_definitions.kind=keyword 规则匹配正文/HTML 源码/URL。
	// 敏感级规则告警、普通级事件，type=keyword_hit。
	if p.engineEnabled("keyword") && len(p.KeywordRules) > 0 {
		_, text := extractHTMLText(html)
		for _, kr := range p.KeywordRules {
			re, err := regexp.Compile(kr.Pattern)
			if err != nil {
				continue
			}
			// s_regex 过滤：命中过滤正则则整条规则跳过。
			if kr.SRegex != "" {
				if sre, err := regexp.Compile(kr.SRegex); err == nil && sre.MatchString(text) {
					continue
				}
			}
			// 匹配正文 + HTML 源码 + URL 三通道。
			var found []string
			for _, m := range re.FindAllString(text, 5) {
				if len(found) >= 5 {
					break
				}
				found = append(found, m)
			}
			if len(found) < 5 {
				for _, m := range re.FindAllString(html, 5) {
					if len(found) >= 5 {
						break
					}
					if !containsStr(found, m) {
						found = append(found, m)
					}
				}
			}
			if len(found) < 5 {
				for _, m := range re.FindAllString(pageURL, 5) {
					if len(found) >= 5 {
						break
					}
					if !containsStr(found, m) {
						found = append(found, m)
					}
				}
			}
			if len(found) == 0 {
				continue
			}
			sev := engines.SeverityMedium
			if kr.Sensitive {
				sev = engines.SeverityHigh
			}
			out = append(out, findingPayload{
				EngineName: "content_security", Type: "keyword_hit", Severity: sev,
				Title:       "命中关键词: " + kr.Name,
				Description: "页面命中关键词规则「" + kr.Name + "」，命中词: " + truncate(strings.Join(found, "、"), 120),
				URL:         pageURL, Confidence: 0.85,
				Extra: map[string]any{
					"rule_id": kr.ID, "rule_name": kr.Name, "group": kr.Group,
					"matches": found, "sensitive": kr.Sensitive,
				},
			})
		}
	}

	// 图片内容 OCR 识别（image_ocr）：页面图片 tesseract OCR，识别文本复核敏感词/提取 URL。
	if p.engineEnabled("image_ocr") {
		results := runImageOCR(ctx, pageURL, html, nil)
		out = append(out, composeImageOCRFinding(pageURL, results)...)
	}

	// 外链发现（external_link）：解析页面全部外链，白名单过滤 + 恶意域名库命中 + 域名相似度检测，
	// 清单回传 Master 维护基线，新增/移除/域名变更时由 Master 生成 finding。
	if p.engineEnabled("external_link") {
		links := evaluateExternalLinks(extractExternalLinks(pageURL, html), pageHostOf(pageURL), p.DomainRules)
		suspicious := 0
		for _, l := range links {
			if l.Suspicious {
				suspicious++
			}
		}
		out = append(out, findingPayload{
			EngineName: "content_security", Type: "external_link", Severity: engines.SeverityInfo,
			Title:       "外链清单",
			Description: fmt.Sprintf("页面外链清单（共 %d 条，可疑 %d 条）", len(links), suspicious),
			URL:         pageURL, Confidence: 0.99,
			Extra: map[string]any{
				"external_links":   links,
				"external_link_count": len(links),
				"suspicious_count": suspicious,
			},
		})
	}

	// 内容完整性指纹（content_integrity）：计算标题/正文/HTML 三通道 Hash 回传，
	// Master 侧对 importance=high 资产建立基线并周期比对，变更时生成篡改 finding。
	if p.engineEnabled("content_integrity") {
		title, text := extractHTMLText(html)
		out = append(out, findingPayload{
			EngineName: "content_security", Type: "content_integrity", Severity: engines.SeverityInfo,
			Title:       "内容完整性指纹",
			Description: "页面内容指纹采集（供篡改基线比对）",
			URL:         pageURL, Confidence: 0.99,
			Extra: map[string]any{
				"content_fingerprint": map[string]string{
					"title_hash": sha256Hex([]byte(title)),
					"text_hash":  sha256Hex([]byte(text)),
					"body_hash":  sha256Hex(body),
				},
				"fingerprint_version": "v1",
			},
		})
	}
	return out
}

// containsStr 判断字符串切片是否包含目标。
func containsStr(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// truncate 按 rune 截断字符串（超长加省略号）。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// formatHeaders 将 http.Header 序列化为 key: value 文本。
func formatHeaders(hdr http.Header) string {
	var sb strings.Builder
	for k, vs := range hdr {
		for _, v := range vs {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// extractHTMLText 提取 HTML 的 <title> 与正文纯文本（去除标签/脚本/样式）。
func extractHTMLText(html string) (title, text string) {
	// go1.26 regexp（RE2）不支持反向引用 \1，script/style/noscript 分开匹配。
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNoScript := regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reTitle := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reTag2 := regexp.MustCompile(`(?s)<[^>]+>`)
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		title = strings.TrimSpace(reTag2.ReplaceAllString(m[1], ""))
	}
	body := reScript.ReplaceAllString(html, " ")
	body = reStyle.ReplaceAllString(body, " ")
	body = reNoScript.ReplaceAllString(body, " ")
	body = reTag2.ReplaceAllString(body, " ")
	body = htmlEntityDecode(body)
	body = regexp.MustCompile(`\s+`).ReplaceAllString(body, " ")
	return title, strings.TrimSpace(body)
}

// extractLinks 提取页面 <a href> 链接（绝对/相对）。
func extractLinks(html string) []string {
	re := regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["']`)
	var out []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		u := strings.TrimSpace(m[1])
		if u == "" || strings.HasPrefix(u, "#") || strings.HasPrefix(u, "javascript:") || strings.HasPrefix(u, "mailto:") {
			continue
		}
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// htmlEntityDecode 解码常用 HTML 实体（正文文本还原）。
func htmlEntityDecode(s string) string {
	repl := strings.NewReplacer(
		"&nbsp;", " ", "&lt;", "<", "&gt;", ">", "&amp;", "&",
		"&quot;", "\"", "&#39;", "'",
	)
	return repl.Replace(s)
}

func severityForStatus(code int) string {
	switch {
	case code >= 500:
		return "high"
	case code >= 400:
		return "medium"
	default:
		return "low"
	}
}

// post HTTP POST + 解析统一响应。
func (w *worker) post(path string, body []byte, out any) error {
	req, err := http.NewRequest(http.MethodPost, w.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.clientID != "" {
		req.Header.Set("X-Client-Id", w.clientID)
		req.Header.Set("X-Client-Secret", w.clientSecret)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	r, err := client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

// uploadEvidenceChunked 将大证据分片上传到 Master（4MB/片，upload_id 断点续传），成功返回 evidence_id。
func (w *worker) uploadEvidenceChunked(ev inlineEvidence) (int64, bool) {
	data, err := base64.StdEncoding.DecodeString(ev.Content)
	if err != nil {
		return 0, false
	}
	uploadID := fmt.Sprintf("%s-%d", ev.SHA256[:16], time.Now().UnixNano())
	const chunkSize = 4 << 20
	total := (len(data) + chunkSize - 1) / chunkSize
	for i := 0; i < total; i++ {
		end := (i + 1) * chunkSize
		if end > len(data) {
			end = len(data)
		}
		body, _ := json.Marshal(map[string]any{
			"upload_id": uploadID, "kind": ev.Kind,
			"total_chunks": total, "chunk_index": i,
			"data": base64.StdEncoding.EncodeToString(data[i*chunkSize : end]),
			"sha256": ev.SHA256,
		})
		var resp struct {
			Code int    `json:"code"`
			Data struct {
				EvidenceID int64 `json:"evidence_id"`
				Complete   bool  `json:"complete"`
			} `json:"data"`
		}
		if err := w.post("/api/v1/worker/evidence", body, &resp); err != nil || resp.Code != 0 {
			return 0, false
		}
		if resp.Data.Complete {
			return resp.Data.EvidenceID, true
		}
	}
	return 0, false
}

func atoi(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ---- 协议结构 ----

type taskPayload struct {
	ID       int64  `json:"id"`
	AssetID  int64  `json:"asset_id"`
	PolicyID int64  `json:"policy_id"`
	Status   string `json:"status"`
}

type policyPayload struct {
	Timeout         int      `json:"timeout"`
	Concurrency     int      `json:"concurrency"`
	FailCount       int      `json:"fail_count"`
	SlowThresholdMS int      `json:"slow_threshold_ms"`
	EnableEvidence  bool     `json:"enable_evidence"`
	Engines         []string `json:"engines"`
	KeywordRules    []keywordRule `json:"keyword_rules"`
	DomainRules     []domainRule  `json:"domain_rules"`

	// 递归扫描配置（随任务下发）。
	ScanDepth        int  `json:"-"`
	ConcurrencyLimit int  `json:"-"`
	AllowStatic      bool `json:"-"`
	SameOrigin       bool `json:"-"`
	CrawlSubpages    bool `json:"-"`
}

// keywordRule 随任务下发的关键词监测规则（rule_definitions.kind=keyword）。
type keywordRule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	Pattern   string `json:"pattern"`   // 匹配正则（FRegex）
	SRegex    string `json:"s_regex"`   // 过滤正则（可空）
	Sensitive bool   `json:"sensitive"` // 敏感级告警 / 普通级事件
	Scope     string `json:"scope"`
}

// engineEnabled 判断引擎开关列表中是否启用某引擎（空列表默认全部启用）。
func (p *policyPayload) engineEnabled(name string) bool {
	if p == nil || len(p.Engines) == 0 {
		return true
	}
	for _, n := range p.Engines {
		if n == name {
			return true
		}
	}
	return false
}

type assetPayload struct {
	ID   int64  `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type findingPayload struct {
	EngineName  string         `json:"engine_name"`
	Type        string         `json:"type"`
	Severity    string         `json:"severity"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	LineNo      int            `json:"line_no"`
	Confidence  float64        `json:"confidence"`
	EvidenceIDs []int64        `json:"evidence_ids"`
	InlineEvidence []inlineEvidence `json:"inline_evidence,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type inlineEvidence struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

type resultPayload struct {
	ResultID     string           `json:"result_id"`
	TaskID       int64            `json:"task_id"`
	Status       string           `json:"status"`
	TaskTimeout  bool             `json:"task_timeout"`
	StoppedByUser bool            `json:"stopped_by_user"`
	Message      string           `json:"message"`
	Progress     int              `json:"progress"`
	Findings     []findingPayload `json:"findings"`
	Evidence     []inlineEvidence `json:"evidence,omitempty"`
	Metrics      map[string]any   `json:"metrics"`
	Discovered   []discoveredAsset `json:"discovered_assets,omitempty"`
}

// ---- HAR 1.2 ----

type har struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Version string     `json:"version"`
	Creator harCreator `json:"creator"`
	Entries []harEntry `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            int64       `json:"time"`
	Request         harRequest  `json:"request"`
	Response        harResponse `json:"response"`
}

type harRequest struct {
	Method      string   `json:"method"`
	URL         string   `json:"url"`
	HTTPVersion string   `json:"httpVersion"`
	Headers     []harKV  `json:"headers"`
	HeadersSize int      `json:"headersSize"`
	BodySize    int      `json:"bodySize"`
}

type harResponse struct {
	Status      int       `json:"status"`
	StatusText  string    `json:"statusText"`
	HTTPVersion string    `json:"httpVersion"`
	Headers     []harKV   `json:"headers"`
	Content     harContent `json:"content"`
	RedirectURL string    `json:"redirectURL"`
	HeadersSize int       `json:"headersSize"`
	BodySize    int       `json:"bodySize"`
}

type harKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

// buildHAR 从一次探测响应组装 HAR 1.2，gzip 压缩后返回 bytes。
func buildHAR(url string, status int, hdr http.Header, body []byte, ms int64, started time.Time) []byte {
	h := &har{Log: harLog{
		Version: "1.2",
		Creator: harCreator{Name: "cinsight-worker", Version: "0.1.0"},
	}}
	reqH := make([]harKV, 0, 2)
	reqH = append(reqH, harKV{Name: "Host", Value: hostOf(url)})
	reqH = append(reqH, harKV{Name: "Accept", Value: "*/*"})
	respH := make([]harKV, 0, len(hdr))
	for k, vs := range hdr {
		for _, v := range vs {
			respH = append(respH, harKV{Name: k, Value: v})
		}
	}
	h.Log.Entries = append(h.Log.Entries, harEntry{
		StartedDateTime: started.Format(time.RFC3339),
		Time:            ms,
		Request:         harRequest{Method: "GET", URL: url, HTTPVersion: "HTTP/1.1", Headers: reqH, HeadersSize: -1, BodySize: 0},
		Response: harResponse{
			Status: status, StatusText: http.StatusText(status), HTTPVersion: "HTTP/1.1",
			Headers: respH, RedirectURL: "", HeadersSize: -1,
			BodySize: len(body),
			Content:  harContent{Size: len(body), MimeType: sniffMIME(body), Text: string(body)},
		},
	})
	raw, err := json.Marshal(h)
	if err != nil {
		return nil
	}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, _ = gz.Write(raw)
	_ = gz.Close()
	return b.Bytes()
}

func sniffMIME(body []byte) string {
	ct := http.DetectContentType(body)
	return ct
}

// base64Encode / sha256Hex 证据内联编码。
func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---- 断点续扫：任务已抓 URL 本地记录 ----

var crawledDir = "/tmp/cinsight-crawled"

func crawledKey(taskID int64, rawURL string) string {
	return fmt.Sprintf("local:crawled:%d:%s", taskID, sha256Hex([]byte(rawURL)))
}

// recordCrawled 记录任务已抓取的 URL（断点续扫：崩溃/重拉后跳过已抓）。
func (w *worker) recordCrawled(taskID int64, rawURL string) error {
	if err := os.MkdirAll(crawledDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/%s", crawledDir, crawledKey(taskID, rawURL)), []byte(rawURL), 0o644)
}

// alreadyCrawled 判断 URL 是否已被本任务抓过。
func (w *worker) alreadyCrawled(taskID int64, rawURL string) bool {
	_, err := os.Stat(fmt.Sprintf("%s/%s", crawledDir, crawledKey(taskID, rawURL)))
	return err == nil
}

// ---- outbox 断网结果本地缓存（内存 + 文件落盘） ----
type outbox struct {
	mu    sync.Mutex
	items []string
	file  string
}

func (o *outbox) push(w *worker, body []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.items = append(o.items, string(body))
}

func (o *outbox) flush() {
	o.mu.Lock()
	defer o.mu.Unlock()
	_ = os.WriteFile("/tmp/cinsight-worker-outbox.json", []byte(stringsJoin(o.items, "\n")), 0o644)
}

func stringsJoin(items []string, sep string) string {
	var b bytes.Buffer
	for i, s := range items {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s)
	}
	return b.String()
}
