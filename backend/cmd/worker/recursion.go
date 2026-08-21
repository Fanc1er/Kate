package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// discoveredAsset 递归扫描发现的子资产（写 assets 表，source_type 标注）。
type discoveredAsset struct {
	URL        string `json:"url"`
	SourceType string `json:"source_type"` // js/css/image/video/subdomain/api_path
}

// recursionConfig 递归扫描配置（来自 scan_policies）。
type recursionConfig struct {
	Depth        int
	Concurrency  int
	AllowStatic  bool
	SameOrigin   bool
	CrawlSubpages bool
}

// normalizeURL 归一化 URL：去 fragment、去默认端口、相对引用按 base 解析，返回规范形式。
func normalizeURL(base string, raw string) string {
	raw = strings.TrimSpace(raw)
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	// 空 raw 表示页面自身 URL（仅归一化 base：去 fragment/默认端口）。
	if raw == "" {
		b.Fragment = ""
		if (b.Scheme == "http" && b.Port() == "80") || (b.Scheme == "https" && b.Port() == "443") {
			b.Host = b.Hostname()
		}
		return b.String()
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	ref := b.ResolveReference(u)
	// 仅支持 http/https。
	if ref.Scheme != "http" && ref.Scheme != "https" {
		return ""
	}
	if ref.Scheme == "" {
		ref.Scheme = b.Scheme
	}
	ref.Fragment = ""
	// 去默认端口。
	if (ref.Scheme == "http" && ref.Port() == "80") || (ref.Scheme == "https" && ref.Port() == "443") {
		ref.Host = ref.Hostname()
	}
	return ref.String()
}

var (
	reScript  = regexp.MustCompile(`(?is)<script[^>]*src\s*=\s*["']([^"']+)["']`)
	reLinkCSS = regexp.MustCompile(`(?is)<link[^>]*rel\s*=\s*["']stylesheet["'][^>]*href\s*=\s*["']([^"']+)["']`)
	reLinkCSS2 = regexp.MustCompile(`(?is)<link[^>]*href\s*=\s*["']([^"']+)["'][^>]*rel\s*=\s*["']stylesheet["']`)
	reImg    = regexp.MustCompile(`(?is)<img[^>]*src\s*=\s*["']([^"']+)["']`)
	reImgSrc = regexp.MustCompile(`(?is)<img[^>]*srcset\s*=\s*["']([^"']+)["']`)
	reVideo  = regexp.MustCompile(`(?is)<video[^>]*src\s*=\s*["']([^"']+)["']`)
	reAudio  = regexp.MustCompile(`(?is)<audio[^>]*src\s*=\s*["']([^"']+)["']`)
	reSource = regexp.MustCompile(`(?is)<source[^>]*src\s*=\s*["']([^"']+)["']`)
	reForm   = regexp.MustCompile(`(?is)<form[^>]*action\s*=\s*["']([^"']+)["']`)
)

var staticExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".svg": true, ".ico": true, ".bmp": true, ".css": true, ".js": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wav": true, ".ogg": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".pdf": true,
}

// isStaticFile 判断 URL 路径是否指向静态文件资源。
func isStaticFile(u string) bool {
	pu, err := url.Parse(u)
	if err != nil {
		return false
	}
	path := strings.ToLower(pu.Path)
	// 带查询参数的静态资源（如 ?v=1）也识别扩展名。
	for ext := range staticExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// isStaticURL 策略 allow_static=false 时过滤静态资源 URL（不递归抓取）。
func (rc *recursionConfig) isStaticFiltered(u string) bool {
	if rc.AllowStatic {
		return false
	}
	return isStaticFile(u)
}

// classifyAsset 按 URL 判定资产类型（js/css/image/video/subdomain/api_path）。
// subdomain: 与页面域名不同但同根域的子域。
func classifyAsset(pageURL, assetURL string) string {
	if isStaticFile(assetURL) {
		pu, _ := url.Parse(assetURL)
		path := strings.ToLower(pu.Path)
		switch {
		case strings.HasSuffix(path, ".js"):
			return "js"
		case strings.HasSuffix(path, ".css"):
			return "css"
		case strings.HasSuffix(path, ".mp4"), strings.HasSuffix(path, ".webm"),
			strings.HasSuffix(path, ".mp3"), strings.HasSuffix(path, ".wav"),
			strings.HasSuffix(path, ".ogg"):
			return "video"
		default:
			return "image"
		}
	}
	pu, err := url.Parse(assetURL)
	if err != nil {
		return ""
	}
	if !pu.IsAbs() {
		return ""
	}
	puPath := strings.TrimSuffix(pu.Path, "/")
	// 接口路径：扩展名非静态且路径非单一页（含多段或常见接口关键词）。
	if len(strings.Split(puPath, "/")) > 2 || strings.Contains(puPath, "/api/") {
		return "api_path"
	}
	// 子域名：与页面域名不同但同根域。
	baseHost := hostOf(pageURL)
	if baseHost != "" && pu.Host != baseHost && isSameRootDomain(pu.Host, baseHost) {
		return "subdomain"
	}
	return ""
}

// isSameRootDomain 判断两域名是否同根域（xx.example.com 与 example.com 视为同根）。
func isSameRootDomain(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	if len(partsA) < 2 || len(partsB) < 2 {
		return a == b
	}
	return partsA[len(partsA)-2]+"."+partsA[len(partsA)-1] ==
		partsB[len(partsB)-2]+"."+partsB[len(partsB)-1]
}

// sameOriginOf 判断候选 URL 是否与种子同域/同子域（策略 same_origin）。
func sameOriginOf(seed, candidate string) bool {
	s := hostOf(seed)
	c := hostOf(candidate)
	if s == "" || c == "" {
		return false
	}
	if c == s {
		return true
	}
	// 同子域：候选为种子的子域（sub.example.com 之于 example.com）。
	return strings.HasSuffix(c, "."+s)
}

// extractAssets 从 HTML 提取页面引用的静态资源与接口路径（归一化后去重）。
func extractAssets(pageURL, html string) []discoveredAsset {
	seen := map[string]bool{}
	var out []discoveredAsset
	add := func(raw, sourceType string) {
		u := normalizeURL(pageURL, raw)
		if u == "" || seen[u] {
			return
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return
		}
		st := sourceType
		if st == "" {
			st = classifyAsset(pageURL, u)
		}
		if st == "" {
			return
		}
		seen[u] = true
		out = append(out, discoveredAsset{URL: u, SourceType: st})
	}
	for _, m := range reScript.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "js")
		}
	}
	for _, m := range reLinkCSS.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "css")
		}
	}
	for _, m := range reLinkCSS2.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "css")
		}
	}
	for _, m := range reImg.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "image")
		}
	}
	for _, m := range reImgSrc.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			for _, src := range strings.Split(m[1], ",") {
				f := strings.Fields(strings.TrimSpace(src))
				if len(f) > 0 {
					add(f[0], "image")
				}
			}
		}
	}
	for _, m := range reVideo.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "video")
		}
	}
	for _, m := range reAudio.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "video")
		}
	}
	for _, m := range reSource.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "")
		}
	}
	for _, m := range reForm.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], "api_path")
		}
	}
	return out
}

// recursiveCrawl 以种子 URL 递归抓取子页面，执行内容子能力并收集发现的资产。
// 返回全部发现资产（js/css/image/video/subdomain/api_path）+ 已抓 URL 数。
func (w *worker) recursiveCrawl(ctx context.Context, seedURL string, p *policyPayload, headers http.Header) ([]discoveredAsset, int, []findingPayload, error) {
	rc := &recursionConfig{
		Depth:        p.ScanDepth,
		Concurrency:  p.ConcurrencyLimit,
		AllowStatic:  p.AllowStatic,
		SameOrigin:   p.SameOrigin,
		CrawlSubpages: p.CrawlSubpages,
	}
	if rc.Depth <= 0 {
		rc.Depth = 2
	}
	if rc.Depth > 5 {
		rc.Depth = 5
	}
	if rc.Concurrency <= 0 {
		rc.Concurrency = 4
	}
	if rc.Concurrency > 32 {
		rc.Concurrency = 32
	}
	// crawl_subpages=false 时深度强制 1（仅种子页）。
	if !rc.CrawlSubpages {
		rc.Depth = 1
	}

	var mu sync.Mutex
	seen := map[string]bool{seedURL: true}
	var assets []discoveredAsset
	var findings []findingPayload
	crawled := 0
	// 深度 BFS 队列：每层一批。
	current := []string{seedURL}
	for depth := 1; depth <= rc.Depth && len(current) > 0; depth++ {
		if ctx.Err() != nil {
			break
		}
		type result struct {
			url     string
			html    []byte
			headers http.Header
			next    []string
			assets  []discoveredAsset
			finds   []findingPayload
		}
		results := make([]result, len(current))
		sem := make(chan struct{}, rc.Concurrency)
		var wg sync.WaitGroup
		for i, u := range current {
			wg.Add(1)
			go func(i int, u string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				// 递归子页面（非种子页）：仅抓取页面，不再执行可用性重探。
				res := result{url: u}
				client := &http.Client{}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
				if err != nil {
					results[i] = res
					return
				}
				for k, vs := range headers {
					for _, v := range vs {
						req.Header.Add(k, v)
					}
				}
				r, err := client.Do(req)
				if err != nil {
					results[i] = res
					return
				}
				body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				r.Body.Close()
				// 静态文件或非 HTML 直接跳过递归。
				ctype := r.Header.Get("Content-Type")
				if !strings.Contains(ctype, "text/html") && !strings.Contains(ctype, "application/xhtml") {
					results[i] = res
					return
				}
				res.html = body
				res.headers = r.Header
				html := string(body)
				res.assets = extractAssets(u, html)
				// 提取子页面链接用于下一层递归。
				for _, raw := range extractLinks(html) {
					nu := normalizeURL(u, raw)
					if nu == "" {
						continue
					}
					mu.Lock()
					dup := seen[nu]
					if !dup {
						seen[nu] = true
					}
					mu.Unlock()
					if dup {
						continue
					}
					// 静态文件与无效类型过滤（allow_static=false 时）。
					if rc.isStaticFiltered(nu) {
						continue
					}
					// 同域限制（same_origin=true 时仅同域/同子域递归）。
					if rc.SameOrigin && !sameOriginOf(seedURL, nu) {
						continue
					}
					res.next = append(res.next, nu)
				}
				// 子页面内容子能力（crawl_subpages=true 时）。
				if rc.CrawlSubpages {
					res.finds = runContentEngines(ctx, u, body, p, r.Header)
				}
				results[i] = res
			}(i, u)
		}
		wg.Wait()
		// 汇总本层结果。
		var next []string
		for _, res := range results {
			if res.url == "" {
				continue
			}
			crawled++
			assets = append(assets, res.assets...)
			findings = append(findings, res.finds...)
			next = append(next, res.next...)
		}
		current = next
	}
	return assets, crawled, findings, nil
}
