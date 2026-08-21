package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/engines"
)

// imageOCR 图片 OCR 识别：下载页面图片 → tesseract OCR → 敏感词复核 / URL 提取。
// 超时或引擎不可用降级跳过，不阻断任务。

// ocrMaxBytes 单图下载大小上限（5MB）。
const ocrMaxBytes = 5 << 20

// ocrTimeout 单图 OCR 超时。
const ocrTimeout = 15 * time.Second

var reOCRImg = regexp.MustCompile(`(?is)<img[^>]*src\s*=\s*["']([^"']+)["']`)

// ocrResult 单图识别结果。
type ocrResult struct {
	ImageURL    string   `json:"image_url"`
	Text        string   `json:"text"`
	Confidence  float64  `json:"confidence"`
	Source      string   `json:"source"` // regex / ai（AI 未配置时回退 regex）
	SensitiveHits []string `json:"sensitive_hits,omitempty"`
	ExtractedURLs []string `json:"extracted_urls,omitempty"`
	Skipped     bool     `json:"skipped,omitempty"` // 超时/引擎不可用降级
	AIClassify  string   `json:"ai_classify,omitempty"` // AI 分类命中（黄/赌/毒/政），空=未命中/未配置
}

// runImageOCR 对页面内图片执行 OCR 识别，返回识别结果列表。
func runImageOCR(ctx context.Context, pageURL, html string, imgPat []string) []ocrResult {
	var out []ocrResult
	seen := map[string]bool{}
	// 图片 URL 提取 + 白名单模式过滤。
	for _, m := range reOCRImg.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		u := normalizeURL(pageURL, m[1])
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if !isOCRExt(u) {
			continue
		}
		if len(imgPat) > 0 && !matchAny(imgPat, u) {
			continue
		}
		res := ocrImage(ctx, u)
		if res != nil {
			out = append(out, *res)
		}
	}
	return out
}

// isOCRExt 判断图片扩展名是否支持 OCR。
func isOCRExt(u string) bool {
	lower := strings.ToLower(u)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func matchAny(pats []string, s string) bool {
	for _, p := range pats {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ocrImage 下载并识别单图。
func ocrImage(ctx context.Context, u string) *ocrResult {
	ctx2, cancel := context.WithTimeout(ctx, ocrTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx2, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0.0.0")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, ocrMaxBytes))
	if err != nil || len(body) == 0 {
		return nil
	}
	if !isImageBytes(body) {
		return nil
	}
	text, conf, err := tesseractOCR(ctx2, body)
	if err != nil {
		// 超时/引擎不可用降级跳过。
		return &ocrResult{ImageURL: u, Skipped: true, Source: "regex"}
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	res := &ocrResult{ImageURL: u, Text: truncate(strings.TrimSpace(text), 1000), Confidence: conf, Source: "regex"}
	// URL 提取：识别文本中的 http(s) URL 交外链核验。
	reURL := regexp.MustCompile(`https?://[^\s"'<>]+`)
	res.ExtractedURLs = reURL.FindAllString(res.Text, -1)
	if len(res.ExtractedURLs) > 5 {
		res.ExtractedURLs = res.ExtractedURLs[:5]
	}
	// 敏感词复核：识别文本命中敏感词库记录命中词。
	engine := engines.NewSensitiveWordEngine()
	words := map[string]bool{}
	for _, f := range engine.Match(u, "", res.Text, nil) {
		if w, ok := f.Extra["word"].(string); ok && w != "" {
			words[w] = true
		}
	}
	for w := range words {
		res.SensitiveHits = append(res.SensitiveHits, w)
	}
	// AI 分类复核：识别文本做涉黄赌毒政分类（未配置 LLM 静默）。
	if cfg := loadAIConfig(); cfg.enabled() {
		if cat, _ := aiClassify(ctx2, cfg, u, res.Text); cat != "" {
			res.AIClassify = cat
			res.Source = "ai"
		}
	}
	return res
}
// isImageBytes 简易魔数判断图片类型。
func isImageBytes(b []byte) bool {
	if len(b) < 8 {
		return false
	}
	// JPEG/PNG/GIF/WebP/BMP 魔数。
	if b[0] == 0xFF && b[1] == 0xD8 {
		return true
	}
	if bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G'}) {
		return true
	}
	if bytes.HasPrefix(b, []byte("GIF8")) {
		return true
	}
	if bytes.HasPrefix(b, []byte("RIFF")) && bytes.HasPrefix(b[8:], []byte("WEBP")) {
		return true
	}
	if bytes.HasPrefix(b, []byte("BM")) {
		return true
	}
	return false
}

// tesseractOCR 调用 tesseract CLI 识别图片文本（eng+chi_sim）。
func tesseractOCR(ctx context.Context, img []byte) (string, float64, error) {
	tmp, err := os.CreateTemp("", "ocr-*.png")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(img); err != nil {
		tmp.Close()
		return "", 0, err
	}
	tmp.Close()
	cmd := exec.CommandContext(ctx, "tesseract", tmp.Name(), "stdout", "-l", "eng+chi_sim", "--psm", "3")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", 0, err
	}
	text := strings.TrimSpace(out.String())
	conf := 0.7
	if text == "" {
		conf = 0
	}
	return text, conf, nil
}

// composeImageOCRFinding 组装 OCR finding（命中敏感词/URL 才产出，否则静默）。
func composeImageOCRFinding(pageURL string, results []ocrResult) []findingPayload {
	var out []findingPayload
	for _, r := range results {
		if r.Skipped || r.Text == "" {
			continue
		}
		reason := []string{}
		if len(r.SensitiveHits) > 0 {
			reason = append(reason, "敏感词")
		}
		if len(r.ExtractedURLs) > 0 {
			reason = append(reason, "URL")
		}
		if r.AIClassify != "" {
			reason = append(reason, "AI 分类: "+r.AIClassify)
		}
		if len(reason) == 0 {
			// 仅含 URL / 敏感词 / AI 分类命中的才产出，纯文本图片静默。
			continue
		}
		out = append(out, findingPayload{
			EngineName: "content_security", Type: "image_ocr", Severity: engines.SeverityMedium,
			Title:       "图片内容识别: " + r.ImageURL,
			Description: fmt.Sprintf("图片 OCR 识别出 %s（置信度 %.2f），图片: %s",
				strings.Join(reason, "、"), r.Confidence, r.ImageURL),
			URL: pageURL, Confidence: 0.7,
			Extra: map[string]any{
				"image_url": r.ImageURL, "ocr_text": truncate(r.Text, 500),
				"confidence": r.Confidence, "source": r.Source,
				"sensitive_hits": r.SensitiveHits, "extracted_urls": r.ExtractedURLs,
				"ai_classify":    r.AIClassify,
			},
		})
	}
	return out
}
