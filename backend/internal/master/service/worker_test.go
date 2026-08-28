package service

import (
	"slices"
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestExpandEngineSwitches(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "高级开关展开为细粒度引擎名",
			input: []string{"availability", "sensitive", "content"},
			want:  []string{"availability", "sensitive_word", "sensitive_info", "ai_classify", "dead_link", "keyword", "image_ocr", "external_link", "content_integrity"},
		},
		{
			name:  "细粒度引擎名原样保留",
			input: []string{"multi_ua", "sensitive_word"},
			want:  []string{"multi_ua", "sensitive_word"},
		},
		{
			name:  "空输入",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "重复去重",
			input: []string{"content", "image_ocr", "content"},
			want:  []string{"ai_classify", "dead_link", "keyword", "image_ocr", "external_link", "content_integrity"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandEngineSwitches(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expandEngineSwitches(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Fatalf("expandEngineSwitches(%v)[%d] = %q, want %q; full: %v", tt.input, i, got[i], w, got)
				}
			}
		})
	}
}

func TestExpandEngineSwitchesContainsWorkerEngineNames(t *testing.T) {
	input := []string{"availability", "vuln_scan", "dns", "sensitive", "webshell", "content", "intel", "subdomain", "port", "tech_stack"}
	got := expandEngineSwitches(input)
	for _, name := range []string{"availability", "sensitive_word", "sensitive_info", "ai_classify", "dead_link", "keyword", "image_ocr", "external_link", "content_integrity"} {
		if !slices.Contains(got, name) {
			t.Fatalf("expandEngineSwitches 结果缺失 Worker 细粒度引擎 %q: %v", name, got)
		}
	}
}

// 降噪规则语义：whitelist_ip 仅对 availability 引擎生效且按 host 精确匹配。
func TestIsNoisyWhitelistIPScope(t *testing.T) {
	gdb := newTestDB(t)
	rules := []models.NoiseRule{
		{Type: "whitelist_ip", Config: `{"ip":"127.0.0.1"}`, Enabled: "true"},
		{Type: "ignore_type", Config: `{"event_type":"port_service"}`, Enabled: "true"},
	}
	for i := range rules {
		if err := gdb.Create(&rules[i]).Error; err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}
	s := NewWorkerService(gdb, nil, nil, NewHub(), nil, 10)

	cases := []struct {
		rawURL string
		engine string
		want   bool
		note   string
	}{
		{"http://127.0.0.1:9090/", "availability", true, "可用性引擎应被回环白名单降噪"},
		{"http://127.0.0.1:9090/", "vuln_scan", false, "漏洞发现照常告警"},
		{"http://127.0.0.1:9090/backdoor.php", "webshell", false, "后门发现照常告警"},
		{"http://evil127.0.0.1.example.com/", "availability", false, "子串误伤 URL 不应命中"},
		{"http://127.0.0.2/", "availability", false, "不同 IP 不应命中"},
		{"http://any.example.com/", "port_service", true, "ignore_type 按引擎名降噪"},
	}
	for _, c := range cases {
		if got := s.isNoisy(c.rawURL, c.engine); got != c.want {
			t.Errorf("isNoisy(%q,%q)=%v want %v (%s)", c.rawURL, c.engine, got, c.want, c.note)
		}
	}

	// 禁用规则后不再降噪。
	gdb.Model(&models.NoiseRule{}).Where("type = ?", "whitelist_ip").Update("enabled", "false")
	if s.isNoisy("http://127.0.0.1/", "availability") {
		t.Error("disabled rule should not降噪")
	}
}

// 情报 CVE 命中应进漏洞库并按真实 CVE 编号聚合。
func TestAggregateVulnIntelCVE(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, NewResultAssessor(gdb))
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)
	asset := models.Asset{Name: "a", URL: "http://x.example.com", Status: "active"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	mk := func(resultID string) WorkerFinding {
		return WorkerFinding{
			EngineName: "intelligence", Type: "cve_match", Severity: "high",
			Title: "命中 CVE 情报：CVE-2023-44487", Description: "d",
			URL: "http://x.example.com", Confidence: 0.9,
			Extra: map[string]any{"cve_id": "CVE-2023-44487"},
		}
	}
	for _, rid := range []string{"r1", "r2", "r3"} {
		if err := s.processFinding(1, asset.ID, rid, mk(rid)); err != nil {
			t.Fatalf("processFinding: %v", err)
		}
	}

	var vulns []models.Vulnerability
	gdb.Where("asset_id = ?", asset.ID).Find(&vulns)
	if len(vulns) != 1 {
		t.Fatalf("vuln rows = %d, want 1 (aggregated)", len(vulns))
	}
	if vulns[0].CVEID != "CVE-2023-44487" {
		t.Fatalf("cve_id = %q, want real CVE id", vulns[0].CVEID)
	}
	// 高危同时应产出告警。
	var alerts []models.Alert
	gdb.Where("asset_id = ?", asset.ID).Find(&alerts)
	if len(alerts) != 3 || alerts[0].AlertType != "intel" {
		t.Fatalf("alerts = %d type=%s, want 3 intel alerts", len(alerts), alerts[0].AlertType)
	}
}
