package service

import (
	"slices"
	"testing"
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
