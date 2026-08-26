package engines

import (
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

func TestWebshellCheckContentSystem(t *testing.T) {
	fs := CheckContent(`<?php system($_GET['cmd']); ?>`)
	var hasPHP bool
	for _, f := range fs {
		if f.Type == "webshell_php" {
			hasPHP = true
			break
		}
	}
	if !hasPHP {
		t.Fatalf("PHP 危险函数组合应命中 webshell_php")
	}
}

func TestWebshellCheckContentClean(t *testing.T) {
	fs := CheckContent(`<html><body><h1>正常页面</h1><p>这是正常内容</p></body></html>`)
	if len(fs) != 0 {
		t.Fatalf("干净内容不应命中, got %d", len(fs))
	}
}
