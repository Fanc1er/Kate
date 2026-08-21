package main

import (
	"net/url"
	"regexp"
	"strings"
)

// externalLinkType 外链类型。
const (
	linkTypeResource = "external_resource" // 外部资源：js/css/图片/音视频
	linkTypeOutbound  = "outbound_link"    // 出站链接：<a href> 指向外部域
	linkTypeThirdParty = "third_party_domain" // 第三方域名：同页引用其他域资源
)

// externalLink 外链记录。
type externalLink struct {
	URL         string `json:"url"`
	Type        string `json:"type"`
	SourcePage  string `json:"source_page"`
	Domain      string `json:"domain"`
	Suspicious  bool   `json:"suspicious"`
	SuspiciousReason string `json:"suspicious_reason,omitempty"`
}

// domainRule 域名规则（白名单/恶意域名库），由 Master 随任务下发。
type domainRule struct {
	Kind    string `json:"kind"` // domain_whitelist / malicious_domain
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Sensitive bool `json:"sensitive"`
}

var reLink = regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["']`)

// extractExternalLinks 解析页面全部外链：出站 <a href> + 外部资源（js/css/图片/音视频）+ 第三方域名。
// 返回去重后的外链清单；pageURL 用于解析相对引用与同源判定。
func extractExternalLinks(pageURL, html string) []externalLink {
	page := normalizeURL(pageURL, "")
	if page == "" {
		return nil
	}
	pageHost, pageRoot := domainParts(page)
	seen := map[string]bool{}
	var out []externalLink
	add := func(raw, typ string) {
		u := normalizeURL(pageURL, raw)
		if u == "" {
			return
		}
		host, root := domainParts(u)
		// 同源（同根域）链接不算外部。
		if root == pageRoot {
			return
		}
		if seen[u] {
			return
		}
		seen[u] = true
		l := externalLink{URL: u, Type: typ, SourcePage: page, Domain: host}
		if root != pageRoot && host != pageHost {
			// 第三方域名类型：出站链接也归第三方。
			l.Type = linkTypeThirdParty
		}
		out = append(out, l)
	}
	// 出站 <a href>。
	for _, m := range reLink.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "javascript:") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") {
			continue
		}
		add(raw, linkTypeOutbound)
	}
	// 外部资源：复用资产提取正则。
	for _, m := range reScript.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], linkTypeResource)
		}
	}
	for _, m := range reLinkCSS.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], linkTypeResource)
		}
	}
	for _, m := range reLinkCSS2.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], linkTypeResource)
		}
	}
	for _, m := range reImg.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			add(m[1], linkTypeResource)
		}
	}
	return out
}

// pageHostOf 返回页面 URL 的 host（用于域名相似度检测的可信域）。
func pageHostOf(raw string) string {
	host, _ := domainParts(raw)
	return host
}

// domainParts 返回 URL 的 host 与根域（二级域名）。
func domainParts(raw string) (host, root string) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}
	host = u.Hostname()
	if host == "" {
		return "", ""
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return host, strings.Join(parts[len(parts)-2:], ".")
	}
	return host, host
}

// domainSimilarity 域名相似度检测：编辑距离 Levenshtein，归一化到 0~1。
// 返回相似度（1 为完全相同）。用于识别仿冒域名（如 gooogle.com）。
func domainSimilarity(a, b string) float64 {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	la, lb := len([]rune(a)), len([]rune(b))
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if []rune(a)[i-1] != []rune(b)[j-1] {
				cost = 1
			}
			dp[i][j] = min3(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
		}
	}
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dp[la][lb])/float64(maxLen)
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// evaluateExternalLinks 外链评估：白名单过滤 + 恶意域名库命中 + 域名相似度检测。
// 返回带可疑标记的外链清单。pageHost 为种子资产 host。
func evaluateExternalLinks(links []externalLink, pageHost string, rules []domainRule) []externalLink {
	var whitelist, malicious []string
	for _, r := range rules {
		if r.Kind == "domain_whitelist" && r.Pattern != "" {
			whitelist = append(whitelist, strings.ToLower(r.Pattern))
		}
		if r.Kind == "malicious_domain" && r.Pattern != "" {
			malicious = append(malicious, strings.ToLower(r.Pattern))
		}
	}
	for i := range links {
		domain := strings.ToLower(links[i].Domain)
		if isWhitelisted(domain, whitelist) {
			links[i].Suspicious = false
			continue
		}
		reason := ""
		// 恶意域名库命中。
		for _, m := range malicious {
			if domain == m || strings.HasSuffix(domain, "."+m) {
				reason = "malicious_domain:" + m
				break
			}
		}
		// 域名相似度检测（仿冒可信域，相似度 ≥0.85 且非自身）。
		if reason == "" && pageHost != "" {
			for _, trust := range []string{pageHost, rootOf(pageHost)} {
				if trust == "" {
					continue
				}
				if sim := domainSimilarity(domain, strings.ToLower(trust)); sim >= 0.85 && domain != strings.ToLower(trust) {
					reason = "similar_domain"
					break
				}
			}
		}
		if reason != "" {
			links[i].Suspicious = true
			links[i].SuspiciousReason = reason
		}
	}
	return links
}

func rootOf(host string) string {
	_, root := domainParts("http://" + host)
	return root
}

func isWhitelisted(domain string, whitelist []string) bool {
	for _, w := range whitelist {
		if domain == w || strings.HasSuffix(domain, "."+w) {
			return true
		}
	}
	return false
}
