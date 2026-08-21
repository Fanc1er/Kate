package main

import (
	"os"
	"path/filepath"
	"testing"
)

// tempCrawledDir 将 crawledDir 重定向到临时目录并返回恢复函数。
func tempCrawledDir(t *testing.T) func() {
	t.Helper()
	old := crawledDir
	dir := filepath.Join(t.TempDir(), "crawled")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	crawledDir = dir
	return func() { crawledDir = old }
}

func TestWorkerRecordAndHitCrawled(t *testing.T) {
	defer tempCrawledDir(t)()
	w := &worker{}

	if w.alreadyCrawled(1, "https://example.com/a") {
		t.Fatal("未记录时不应命中")
	}
	if err := w.recordCrawled(1, "https://example.com/a"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if !w.alreadyCrawled(1, "https://example.com/a") {
		t.Fatal("记录后应命中")
	}
}

func TestWorkerCrawledDifferentTaskIsolated(t *testing.T) {
	defer tempCrawledDir(t)()
	w := &worker{}

	if err := w.recordCrawled(1, "https://example.com/a"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if w.alreadyCrawled(2, "https://example.com/a") {
		t.Fatal("不同任务不应命中同一 URL")
	}
}

func TestWorkerCrawledURLHashKey(t *testing.T) {
	// 同一 URL 无论查询参数顺序，key 稳定（基于完整原始串 hash）。
	if crawledKey(1, "https://example.com/a?x=1") == crawledKey(1, "https://example.com/a?x=2") {
		t.Fatal("不同 URL 应生成不同 key")
	}
	if crawledKey(1, "https://example.com/a") != crawledKey(1, "https://example.com/a") {
		t.Fatal("相同 URL 应生成相同 key")
	}
}
