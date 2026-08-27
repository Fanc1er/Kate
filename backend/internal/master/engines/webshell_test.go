package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebshellCheckContentEval(t *testing.T) {
	fs := CheckContent(`<?php eval($_POST['cmd']); ?>`)
	if len(fs) == 0 {
		t.Fatalf("eval 应命中 webshell 特征")
	}
	var hasEval bool
	for _, f := range fs {
		if f.Type == "webshell_pattern" {
			hasEval = true
			break
		}
	}
	if !hasEval {
		t.Fatalf("应命中 webshell_pattern, got %d findings", len(fs))
	}
}

func TestWebshellCheckContentObfuscated(t *testing.T) {
	fs := CheckContent(`<?php eval(base64_decode("c3lzdGVt")); ?>`)
	var hasObf bool
	for _, f := range fs {
		if f.Type == "webshell_obfuscated" {
			hasObf = true
			if f.Severity != SeverityCritical {
				t.Fatalf("severity = %s, want critical", f.Severity)
			}
		}
	}
	if !hasObf {
		t.Fatalf("混淆 eval 应命中 webshell_obfuscated")
	}
}

func TestWebshellCheckContentPlainJSNoFP(t *testing.T) {
	js := `<html><head><script>function init(){eval("1+2");}</script></head><body onload="init()"></body></html>`
	if fs := CheckContent(js); len(fs) != 0 {
		t.Fatalf("普通前端 JS 不应误报, got %d findings", len(fs))
	}
}

func TestWebshellRunSetsTargetURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?php assert($_POST['x']); ?>`))
	}))
	defer srv.Close()

	e := NewWebshellEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if len(fs) == 0 {
		t.Fatalf("PHP 后门页应产出 finding")
	}
	for _, f := range fs {
		if f.URL != srv.URL {
			t.Fatalf("finding URL = %q, want %q", f.URL, srv.URL)
		}
	}
}

func TestWebshellPatternsSystemCritical(t *testing.T) {
	fs := CheckContent(`<?php system('id'); ?>`)
	sawCritical := false
	for _, f := range fs {
		if f.Extra != nil && f.Extra["pattern"] == "系统命令执行函数" && f.Severity == SeverityCritical {
			sawCritical = true
		}
	}
	if !sawCritical {
		t.Fatalf("system 调用应为 critical webshell_pattern")
	}
}

func TestWebshellCheckContentClean(t *testing.T) {
	fs := CheckContent(`<html><body><h1>正常页面</h1><p>这是正常内容</p></body></html>`)
	if len(fs) != 0 {
		t.Fatalf("干净内容不应命中, got %d", len(fs))
	}
}

func TestWebshellCheckContentTooLargeTruncatedInput(t *testing.T) {
	long := "<?php " + strings.Repeat("//pad\n", 100) + "eval($_GET[1]); ?>"
	if len(CheckContent(long)) == 0 {
		t.Fatalf("带填充的 PHP 后门仍应命中")
	}
}
