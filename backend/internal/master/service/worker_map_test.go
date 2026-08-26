package service

import "testing"

func TestMapEventSecurityEngines(t *testing.T) {
	cases := []struct {
		engine, findingType string
		wantEvent           string
	}{
		{"vuln_scan", "path_exposure", "漏洞"},
		{"phishing", "typosquatting", "钓鱼"},
		{"phishing", "no_https", "钓鱼"},
		{"port_service", "port_exposed", "端口暴露"},
		{"webshell", "webshell_pattern", "Webshell"},
		{"hidden_link", "hidden_element", "暗链挂马"},
		{"reputation", "intel_threat_score", "信誉异常"},
		{"intelligence", "cve_match", "情报预警"},
	}
	for _, c := range cases {
		eventType, title := mapEvent(c.engine, c.findingType, "high")
		if eventType != c.wantEvent {
			t.Fatalf("mapEvent(%s,%s) event = %s, want %s (title=%s)", c.engine, c.findingType, eventType, c.wantEvent, title)
		}
		if title == "" {
			t.Fatalf("mapEvent(%s,%s) title 为空", c.engine, c.findingType)
		}
	}
}

func TestMapEventDNSCertPrefix(t *testing.T) {
	for _, ft := range []string{"cert_expired", "cert_expiring_soon", "cert_hostname_mismatch", "cert_not_yet_valid"} {
		eventType, _ := mapEvent("dns_security", ft, "high")
		if eventType != "证书告警" {
			t.Fatalf("mapEvent(dns_security,%s) = %s, want 证书告警", ft, eventType)
		}
	}
	eventType, _ := mapEvent("dns_security", "dns_internal_ip", "high")
	if eventType != "篡改" {
		t.Fatalf("mapEvent(dns_security,dns_internal_ip) = %s, want 篡改", eventType)
	}
}

func TestIsAlertWorthy(t *testing.T) {
	worthy := []struct{ sev, ft string }{
		{"high", "port_exposed"},
		{"critical", "webshell_pattern"},
		{"low", "cert_expiring_soon"},
		{"medium", "path_exposure"},
		{"medium", "hidden_element"},
		{"medium", "dns_resolver_inconsistent"},
		{"medium", "port_exposed"},
	}
	for _, c := range worthy {
		if !isAlertWorthy(c.sev, c.ft) {
			t.Fatalf("isAlertWorthy(%s,%s) 应为 true", c.sev, c.ft)
		}
	}
	notWorthy := []struct{ sev, ft string }{
		{"low", "no_https"},
		{"low", "data_uri"},
		{"info", "direct_ip_host"},
	}
	for _, c := range notWorthy {
		if isAlertWorthy(c.sev, c.ft) {
			t.Fatalf("isAlertWorthy(%s,%s) 应为 false", c.sev, c.ft)
		}
	}
}

func TestSuggestionForNewEngines(t *testing.T) {
	cases := map[string]string{
		"port_service":  "收敛暴露面+补丁加固",
		"dns_security":  "核查 DNS 配置并处理证书",
		"reputation":    "确认域名归属并处置",
		"intelligence":  "按情报修复漏洞并复核",
		"multi_ua":      "排查多线路/边缘节点",
		"unknown_engine": "核查并确认处置",
	}
	for engine, want := range cases {
		if got := suggestionFor(engine); got != want {
			t.Fatalf("suggestionFor(%s) = %s, want %s", engine, got, want)
		}
	}
}

func TestAlertTypeOfNewEngines(t *testing.T) {
	cases := map[string]string{
		"port_service": "port",
		"dns_security": "tamper",
		"reputation":   "content",
		"intelligence": "intel",
	}
	for engine, want := range cases {
		if got := alertTypeOf(engine); got != want {
			t.Fatalf("alertTypeOf(%s) = %s, want %s", engine, got, want)
		}
	}
}
