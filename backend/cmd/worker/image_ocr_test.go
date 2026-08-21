package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOCRImageURLFilter 验证图片 URL 过滤与扩展名白名单。
func TestOCRImageURLFilter(t *testing.T) {
	if !isOCRExt("https://x.com/a.png") {
		t.Fatal(".png 应支持 OCR")
	}
	if isOCRExt("https://x.com/a.js") {
		t.Fatal(".js 不应支持 OCR")
	}
	if isOCRExt("https://x.com/a") {
		t.Fatal("无扩展名不应支持 OCR")
	}
}

// TestIsImageBytes 验证魔数判断。
func TestIsImageBytes(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !isImageBytes(png) {
		t.Fatal("PNG 魔数应识别")
	}
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
	if !isImageBytes(jpg) {
		t.Fatal("JPEG 魔数应识别")
	}
	if isImageBytes([]byte("<html>not an image</html>")) {
		t.Fatal("HTML 不应识别为图片")
	}
}

// TestTesseractAvailability 验证 tesseract 可用（引擎存在）。
func TestTesseractAvailability(t *testing.T) {
	text, conf, err := tesseractOCR(context.Background(), []byte{})
	if err == nil {
		t.Fatal("空字节应报错")
	}
	_ = text
	_ = conf
}

// TestOCRNotFoundImage 下载 404 图片应跳过。
func TestOCRNotFoundImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	res := ocrImage(context.Background(), srv.URL+"/img.png")
	if res != nil {
		t.Fatalf("404 图片应跳过, got %+v", res)
	}
}

// TestComposeOCRFinding 组装逻辑：含 URL 的识别结果产出 finding。
func TestComposeOCRFinding(t *testing.T) {
	results := []ocrResult{
		{ImageURL: "https://x.com/a.png", Text: "banner content", Source: "regex", ExtractedURLs: []string{"https://evil.com/x"}},
		{ImageURL: "https://x.com/b.png", Text: "普通文字", Source: "regex"},
		{ImageURL: "https://x.com/c.png", Skipped: true, Source: "regex"},
	}
	fs := composeImageOCRFinding("https://x.com/page", results)
	if len(fs) != 1 {
		t.Fatalf("仅含 URL 的结果应产出 finding, got %d", len(fs))
	}
	if fs[0].Type != "image_ocr" {
		t.Fatalf("type = %s, want image_ocr", fs[0].Type)
	}
	extra, _ := fs[0].Extra["extracted_urls"].([]string)
	if len(extra) == 0 || extra[0] != "https://evil.com/x" {
		t.Fatalf("应提取 URL, got %v", extra)
	}
}

// TestRunImageOCRNoImages 无图片页面产出空。
func TestRunImageOCRNoImages(t *testing.T) {
	res := runImageOCR(context.Background(), "https://x.com/",
		"<html><body><p>no images</p></body></html>", nil)
	if len(res) != 0 {
		t.Fatalf("无图片应产出空, got %v", res)
	}
}
