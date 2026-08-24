package engines

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const NameVulnScan = "vuln_scan"

// VulnScanEngine 漏洞扫描引擎骨架：HTTP 探针检测常见漏洞模式。
// 实现 POC 库、Fuzzing、参数注入等高级能力需在后续迭代中补充。
type VulnScanEngine struct {
	client *http.Client
}

// NewVulnScanEngine 构造漏洞扫描引擎。
func NewVulnScanEngine() *VulnScanEngine {
	return &VulnScanEngine{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name 返回引擎名。
func (e *VulnScanEngine) Name() string { return NameVulnScan }

// Enabled 策略开关：vuln_scan 未显式关闭即启用。
func (e *VulnScanEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameVulnScan]
	if !ok {
		return true
	}
	return en
}

// Run 执行漏洞扫描：探测常见风险路径与参数注入点。
func (e *VulnScanEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	// 探测常见危险路径（ skeletons for future POC expansion ）
	paths := []string{
		"/.env", "/wp-admin/", "/admin/", "/phpmyadmin/",
		"/.git/config", "/robots.txt", "/sitemap.xml",
		"/api/v1/health", "/actuator/health",
	}
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return findings, ctx.Err()
		default:
		}
		reqURL := target.URL
		if !strings.HasSuffix(reqURL, "/") {
			reqURL += "/"
		}
		reqURL += strings.TrimPrefix(path, "/")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			continue
		}
		resp, err := e.client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		// 敏感路径暴露判定
		if resp.StatusCode == 200 && isSensitivePath(path) {
			findings = append(findings, Finding{
				Type:        "path_exposure",
				Severity:    SeverityMedium,
				Title:       fmt.Sprintf("敏感路径暴露：%s", path),
				Description: fmt.Sprintf("目标 %s 返回 200，路径 %s 可能存在信息泄露", target.URL, path),
				URL:         reqURL,
				Confidence:  0.6,
				LineNo:      0,
				Extra: map[string]any{
					"status_code": resp.StatusCode,
					"path":        path,
				},
			})
		}
	}
	return findings, nil
}

// isSensitivePath 判断是否为敏感路径。
func isSensitivePath(p string) bool {
	sensitive := []string{".env", ".git", "wp-admin", "phpmyadmin", "actuator"}
	for _, s := range sensitive {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
