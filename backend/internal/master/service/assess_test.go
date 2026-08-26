package service

import "testing"

func TestAssessTypeBonus(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)

	bonusTypes := []string{
		"content_integrity", "hidden_element", "external_iframe",
		"typosquatting", "dns_internal_ip", "dns_resolver_inconsistent",
		"port_exposed", "cert_expired", "webshell_pattern", "cve_match",
		"intel_threat_score",
	}
	for _, vt := range bonusTypes {
		score, _, _, detail, err := assessor.Assess(0, "test", "medium", vt, 0.5)
		if err != nil {
			t.Fatalf("Assess(%s) err: %v", vt, err)
		}
		if detail.TypeBonus != 5 {
			t.Fatalf("Assess(%s) TypeBonus = %d, want 5", vt, detail.TypeBonus)
		}
		// base(40)*0.5 + 5 = 25
		if score != 25 {
			t.Fatalf("Assess(%s) score = %d, want 25", vt, score)
		}
	}

	// 非加成类型。
	score, _, _, detail, err := assessor.Assess(0, "test", "medium", "no_https", 0.5)
	if err != nil {
		t.Fatalf("Assess err: %v", err)
	}
	if detail.TypeBonus != 0 {
		t.Fatalf("no_https TypeBonus = %d, want 0", detail.TypeBonus)
	}
	if score != 20 {
		t.Fatalf("no_https score = %d, want 20", score)
	}
}

func TestAssessSeverityBase(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	cases := map[string]int{"critical": 80, "high": 60, "medium": 40, "low": 20, "info": 0}
	for sev, base := range cases {
		score, _, _, detail, err := assessor.Assess(0, "test", sev, "x", 1.0)
		if err != nil {
			t.Fatalf("Assess(%s) err: %v", sev, err)
		}
		if detail.SeverityBase != base {
			t.Fatalf("Assess(%s) base = %d, want %d", sev, detail.SeverityBase, base)
		}
		if score != base {
			t.Fatalf("Assess(%s) score = %d, want %d", sev, score, base)
		}
	}
}
