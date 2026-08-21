package main

import (
	"context"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Screenshotter 无头浏览器截图组件：viewport 渲染 + DOMContentLoaded+2s + PNG 输出。
// 浏览器实例全局复用（sync.Once 惰性启动一次），支持同 URL 缓存、并发信号量池约束、
// 超时降级 screenshot:skipped。
type Screenshotter struct {
	mu      sync.Mutex
	cache   map[string][]byte // url -> png（已成功截图缓存）
	execPath string           // chromium 可执行文件路径（空则自动探测）
	sem     chan struct{}     // 并发池信号量

	browserOnce sync.Once
	browserCtx  context.Context
	cancelAlloc context.CancelFunc
	browserErr  error
}

// NewScreenshotter 构造截图组件。concurrency>0 时启用并发池约束（默认 2）。
// execPath 为空时自动探测 chromium 可执行文件。
func NewScreenshotter(concurrency int, execPath string) *Screenshotter {
	if concurrency <= 0 {
		concurrency = 2
	}
	if execPath == "" {
		execPath = detectChromium()
	}
	return &Screenshotter{
		cache:    map[string][]byte{},
		execPath: execPath,
		sem:      make(chan struct{}, concurrency),
	}
}

// browser 惰性创建无头浏览器实例（sync.Once）。首次 Capture 的 Run 触发分配。
// 浏览器进程复用，不再每次新建 chromium。
func (s *Screenshotter) browser() (context.Context, error) {
	s.browserOnce.Do(func() {
		opts := []chromedp.ExecAllocatorOption{
			chromedp.ExecPath(s.execPath),
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.DisableGPU,
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.WindowSize(1440, 900),
			chromedp.WSURLReadTimeout(60 * time.Second),
			// 低内存环境优化：单进程模式省多进程开销 + 禁用扩展/同步/后台特性。
			chromedp.Flag("single-process", true),
			chromedp.Flag("no-zygote", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-sync", true),
			chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("disable-component-update", true),
			chromedp.Flag("mute-audio", true),
			chromedp.Flag("disable-ipc-flooding-protection", true),
			chromedp.Flag("disable-features", "Translate,InterestFeedContent,AutofillServerCommunication"),
			chromedp.Flag("memory-pressure-off", true),
			// 限制 V8 堆大小（默认可能占数百 MB）。
			chromedp.Flag("js-flags", "--max-old-space-size=256"),
		}
		allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
		bCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
		s.browserCtx = bCtx
		s.cancelAlloc = func() {
			cancelBrowser()
			cancelAlloc()
		}
	})
	return s.browserCtx, s.browserErr
}

// Close 关闭浏览器实例（程序退出时调用）。
func (s *Screenshotter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelAlloc != nil {
		s.cancelAlloc()
		s.cancelAlloc = nil
	}
}

// Capture 抓取页面截图。成功返回 PNG 字节；失败返回 error（超时/渲染失败）。
// 同 URL 缓存命中直接返回缓存；缓存未命中则渲染抓取。
func (s *Screenshotter) Capture(ctx context.Context, url string) ([]byte, error) {
	s.mu.Lock()
	if cached, ok := s.cache[url]; ok {
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	// 并发池约束：获取信号量，超时（10s）则降级跳过。
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, errScreenshotBusy
	}

	bCtx, err := s.browser()
	if err != nil {
		return nil, errScreenshotFailed(err)
	}
	// 每次 Capture 派生独立 tab context，跑完关闭该 tab。
	// 首次调用冷启动 chromium（内存紧张环境可能较慢），给足 90s 超时。
	tabCtx, cancelTab := chromedp.NewContext(bCtx)
	defer cancelTab()
	timeoutCtx, cancelTimeout := context.WithTimeout(tabCtx, 90*time.Second)
	defer cancelTimeout()

	var png []byte
	err = chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 等待 DOMContentLoaded + 2s 渲染稳定。
			if err := chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			<-time.After(2 * time.Second)
			return nil
		}),
		chromedp.FullScreenshot(&png, 100), // v0.16: quality==100 才输出 PNG
	)
	if err != nil || len(png) == 0 {
		return nil, errScreenshotFailed(err)
	}
	// 同 URL 缓存。
	s.mu.Lock()
	s.cache[url] = png
	s.mu.Unlock()
	return png, nil
}

type screenshotErr string

func (e screenshotErr) Error() string { return string(e) }

func errScreenshotFailed(err error) error {
	if err == nil {
		return screenshotErr("screenshot failed")
	}
	return screenshotErr("screenshot failed: " + err.Error())
}

// errScreenshotBusy 并发池已满，降级跳过。
const errScreenshotBusy = screenshotErr("screenshot busy, skip")

// detectChromium 探测 chromium 可执行文件路径（供 worker 配置 execPath）。
func detectChromium() string {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}
