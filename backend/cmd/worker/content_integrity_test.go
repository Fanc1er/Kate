package main

import (
	"context"
	"net/http"
	"testing"
)

func TestContentIntegrityFingerprint(t *testing.T) {
	p := &policyPayload{
		Engines: []string{"content_integrity"},
	}
	fs := runContentEngines(context.Background(), "https://example.com/",
		[]byte("<html><head><title>官网首页</title></head><body>欢迎来到我们的官网</body></html>"),
		p, http.Header{})

	var fp *findingPayload
	for i := range fs {
		if fs[i].Type == "content_integrity" {
			fp = &fs[i]
			break
		}
	}
	if fp == nil {
		t.Fatal("应产出 content_integrity 指纹 finding")
	}
	extra, ok := fp.Extra["content_fingerprint"].(map[string]string)
	if !ok {
		t.Fatalf("extra.content_fingerprint 应为 map[string]string, got %T", fp.Extra["content_fingerprint"])
	}
	if extra["title_hash"] == "" || extra["text_hash"] == "" || extra["body_hash"] == "" {
		t.Fatal("三通道 hash 均不应为空")
	}
	if extra["title_hash"] == extra["body_hash"] {
		t.Fatal("标题/正文/HTML hash 应不同")
	}
}

func TestContentIntegrityFingerprintStable(t *testing.T) {
	p := &policyPayload{Engines: []string{"content_integrity"}}
	html := []byte("<html><head><title>稳定页</title></head><body>内容不变</body></html>")
	fs1 := runContentEngines(context.Background(), "https://example.com/", html, p, http.Header{})
	fs2 := runContentEngines(context.Background(), "https://example.com/", html, p, http.Header{})
	getHash := func(fs []findingPayload, key string) string {
		for _, f := range fs {
			if f.Type == "content_integrity" {
				if fp, ok := f.Extra["content_fingerprint"].(map[string]string); ok {
					return fp[key]
				}
			}
		}
		return ""
	}
	if getHash(fs1, "title_hash") != getHash(fs2, "title_hash") {
		t.Fatal("相同内容标题 hash 应一致")
	}
	if getHash(fs1, "body_hash") != getHash(fs2, "body_hash") {
		t.Fatal("相同内容 body hash 应一致")
	}
}

func TestContentIntegrityFingerprintDisabled(t *testing.T) {
	p := &policyPayload{Engines: []string{"sensitive_word"}}
	fs := runContentEngines(context.Background(), "https://example.com/",
		[]byte("<html><body>x</body></html>"), p, http.Header{})
	for _, f := range fs {
		if f.Type == "content_integrity" {
			t.Fatal("content_integrity 引擎关闭时不应产出指纹")
		}
	}
}
