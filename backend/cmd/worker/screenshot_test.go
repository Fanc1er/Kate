package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// requireScreenshotEnv 截图集成测试依赖真实 chromium，默认跳过；
// 设置 CINSIGHT_TEST_SCREENSHOT=1 显式启用。
func requireScreenshotEnv(t *testing.T) {
	if os.Getenv("CINSIGHT_TEST_SCREENSHOT") != "1" {
		t.Skip("CINSIGHT_TEST_SCREENSHOT=1 未设置，跳过截图集成测试")
	}
	if detectChromium() == "" {
		t.Skip("chromium 不可用，跳过截图集成测试")
	}
}

// TestScreenshotterCapture 集成测试：真实 chromium 无头截图本地页面，输出有效 PNG。
func TestScreenshotterCapture(t *testing.T) {
	requireScreenshotEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>shot</title></head><body><h1>Hello</h1></body></html>")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	shot := NewScreenshotter(2, "")
	defer shot.Close()
	png, err := shot.Capture(ctx, srv.URL)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(png) < 1000 {
		t.Fatalf("PNG 过小: %d bytes", len(png))
	}
	if len(png) < 8 || png[0] != 0x89 || png[1] != 'P' || png[2] != 'N' || png[3] != 'G' {
		t.Fatal("输出不是有效 PNG")
	}
}

// TestScreenshotterCache 同 URL 缓存命中不重复渲染，且缓存字节仍为 PNG。
func TestScreenshotterCache(t *testing.T) {
	requireScreenshotEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body>cached</body></html>")
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	shot := NewScreenshotter(2, "")
	defer shot.Close()
	png1, err := shot.Capture(ctx, srv.URL)
	if err != nil {
		t.Fatalf("first capture: %v", err)
	}
	png2, err := shot.Capture(ctx, srv.URL)
	if err != nil {
		t.Fatalf("second capture: %v", err)
	}
	if string(png1) != string(png2) {
		t.Fatal("缓存命中应返回相同字节")
	}
	if len(png2) < 8 || png2[0] != 0x89 || png2[1] != 'P' || png2[2] != 'N' || png2[3] != 'G' {
		t.Fatal("缓存命中应返回 PNG")
	}
}

// TestScreenshotterUnreachable 不可达 URL 应返回错误（不阻塞主流程）。
func TestScreenshotterUnreachable(t *testing.T) {
	requireScreenshotEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	shot := NewScreenshotter(2, "")
	defer shot.Close()
	_, err := shot.Capture(ctx, "http://127.0.0.1:1/nothing")
	if err == nil {
		t.Fatal("不可达 URL 应返回错误")
	}
}
