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

const NameVulnScan = "vuln_scan"

// vulnProbePaths 敏感路径探测清单：配置/源码/备份类泄露是最常见的信息暴露面。
var vulnProbePaths = []string{
	"/.env", "/.env.local", "/.git/config", "/.git/HEAD",
	"/.svn/entries", "/.htaccess", "/WEB-INF/web.xml",
	"/composer.json", "/package.json", "/.DS_Store",
	"/phpinfo.php", "/info.php", "/adminer.php",
	"/backup.sql", "/db.sql", "/dump.sql",
	"/wp-config.php.bak", "/.aws/credentials",
	"/actuator/env", "/phpmyadmin/", "/admin/",
}

// 路径内容标记：命中标记视为真实泄露（置信度更高），未命中标记在软 404 模式下跳过。
var (
	reEnvPair     = regexp.MustCompile(`(?m)^[A-Za-z0-9_]{3,}\s*=`)
	reGitConfig   = regexp.MustCompile(`(?m)^\[(core|branch|remote|user)\]`)
	reGitHEAD     = regexp.MustCompile(`(?m)^ref:\s*refs/`)
	reWebXML      = regexp.MustCompile(`(?i)<web-app`)
	reSQLDump     = regexp.MustCompile(`(?i)(INSERT INTO|CREATE TABLE)`)
	rePHPInfo     = regexp.MustCompile(`(?i)(phpinfo\(\)|PHP Version \d)`)
	reDSStore     = regexp.MustCompile(`^\x00\x00\x00\x01Bud1`)
	reCredentials = regexp.MustCompile(`(?i)aws_access_key_id`)
)

func vulnContentMarker(path, body string) (bool, string) {
	switch {
	case strings.Contains(path, ".env"):
		return reEnvPair.MatchString(body), "env_kv_pairs"
	case strings.HasPrefix(path, "/.git"):
		return reGitConfig.MatchString(body) || reGitHEAD.MatchString(body), "git_repo_file"
	case strings.HasSuffix(path, ".sql"), strings.HasSuffix(path, ".bak"):
		return reSQLDump.MatchString(body), "db_dump_marker"
	case strings.HasPrefix(path, "/WEB-INF"):
		return reWebXML.MatchString(body), "web_xml_marker"
	case path == "/phpinfo.php" || path == "/info.php":
		return rePHPInfo.MatchString(body), "phpinfo_marker"
	case path == "/.DS_Store":
		return reDSStore.MatchString(body), "ds_store_magic"
	case path == "/.aws/credentials":
		return reCredentials.MatchString(body), "aws_keys"
	case path == "/composer.json":
		return strings.Contains(body, `"require"`), "composer_manifest"
	default:
		return false, ""
	}
}

// VulnScanEngine 漏洞扫描引擎：HTTP 探针检测敏感路径与配置泄露。
// POC 库、参数 Fuzzing 等高级能力需在后续迭代中补充。
type VulnScanEngine struct {
	client *http.Client
}

// NewVulnScanEngine 构造漏洞扫描引擎。
func NewVulnScanEngine() *VulnScanEngine {
	return &VulnScanEngine{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithClient 注入自定义 HTTP 客户端。
func (e *VulnScanEngine) WithClient(c *http.Client) *VulnScanEngine {
	if c != nil {
		e.client = c
	}
	return e
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

type probeResult struct {
	path      string
	statusOK  bool
	body      string
	markerHit bool
	marker    string
}

// Run 执行敏感路径探测：
// 1. 基线路径判定软 404（任意路径都回 200 的服务器）；
// 2. 软 404 场景下要求响应体命中内容标记才判泄露；
// 3. 正常场景 200 + 敏感路径即上报，命中内容标记时提升置信度与等级。
func (e *VulnScanEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	soft404 := false

	baseURL := fmt.Sprintf("%s/__cinsight_probe_%d", strings.TrimSuffix(target.URL, "/"), time.Now().UnixNano())
	if st, baseBody := e.probe(ctx, baseURL); st {
		soft404 = len(baseBody) > 0
	}

	for _, path := range vulnProbePaths {
		select {
		case <-ctx.Done():
			return findings, ctx.Err()
		default:
		}
		reqURL := joinPath(target.URL, path)
		statusOK, body := e.probe(ctx, reqURL)
		if !statusOK {
			continue
		}
		markerHit, marker := vulnContentMarker(path, body)
		if soft404 && !markerHit {
			continue
		}
		confidence := 0.6
		title := fmt.Sprintf("敏感路径暴露：%s", path)
		description := fmt.Sprintf("目标 %s 返回 200，路径 %s 可能存在信息泄露", target.URL, path)
		extra := map[string]any{"status_code": http.StatusOK, "path": path}
		if markerHit {
			confidence = 0.85
			description += "，且响应体含特征内容"
			extra["marker"] = marker
			findings = append(findings, Finding{
				Type: "path_exposure", Severity: SeverityHigh,
				Title: title + "（已确认）", Description: description,
				URL: reqURL, Confidence: confidence, Extra: extra,
			})
			continue
		}
		findings = append(findings, Finding{
			Type: "path_exposure", Severity: SeverityMedium,
			Title: title, Description: description,
			URL: reqURL, Confidence: confidence, Extra: extra,
		})
	}
	return findings, nil
}

func (e *VulnScanEngine) probe(ctx context.Context, url string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, ""
	}
	req.Header.Set("User-Agent", "CInsight-Scanner/0.1")
	resp, err := e.client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode == http.StatusOK, string(body)
}

func joinPath(base, p string) string {
	return strings.TrimSuffix(base, "/") + p
}

// isSensitivePath 判断路径是否属敏感类别。
func isSensitivePath(p string) bool {
	sensitive := []string{
		".env", ".git", ".svn", ".htaccess", "web.xml", ".bak",
		".sql", "phpinfo", "adminer", "phpmyadmin", "actuator",
		".aws", ".DS_Store", "composer.json", "/admin",
	}
	for _, s := range sensitive {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
