package webfetch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ChromiumConfig 配置浏览器渲染的超时时间。
type ChromiumConfig struct {
	Timeout time.Duration
}

// chromiumBrowser 管理 Chromium 渲染与公网请求校验。
type chromiumBrowser struct {
	config  ChromiumConfig
	network *publicNetwork
}

// NewChromiumBrowser 创建用于补充 HTML 页面内容的浏览器渲染器。
func NewChromiumBrowser(config ChromiumConfig) Browser {
	return newChromiumBrowser(config)
}

// newChromiumBrowser 创建带默认超时与公网访问策略的浏览器渲染器。
func newChromiumBrowser(config ChromiumConfig) *chromiumBrowser {
	if config.Timeout <= 0 {
		config.Timeout = defaultBrowserTimeout
	}
	return &chromiumBrowser{config: config, network: newPublicNetwork()}
}

// Preflight 检查当前环境能否启动 Chromium 并打开空白页面。
func (b *chromiumBrowser) Preflight(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, b.config.Timeout)
	defer cancel()

	allocatorCtx, allocatorCancel, err := b.allocator(ctx)
	if err != nil {
		return err
	}
	defer allocatorCancel()
	tabCtx, tabCancel := chromedp.NewContext(allocatorCtx)
	defer tabCancel()

	if err := chromedp.Run(tabCtx,
		chromedp.Navigate("about:blank"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("start Chromium: %w", err)
	}
	return nil
}

// Render 渲染页面并返回正文、最终地址和候选链接。
func (b *chromiumBrowser) Render(ctx context.Context, rawURL string) (*RenderedPage, error) {
	if err := b.network.validateURL(ctx, rawURL); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, b.config.Timeout)
	defer cancel()

	allocatorCtx, allocatorCancel, err := b.allocator(ctx)
	if err != nil {
		return nil, &Error{Status: "web_unavailable"}
	}
	defer allocatorCancel()
	tabCtx, tabCancel := chromedp.NewContext(allocatorCtx)
	defer tabCancel()

	blocked := make(chan error, 1)
	chromedp.ListenTarget(tabCtx, func(event any) {
		paused, ok := event.(*fetch.EventRequestPaused)
		if !ok || paused.Request == nil {
			return
		}
		go b.handlePaused(tabCtx, paused, blocked)
	})

	var content, finalURL string
	var links struct {
		Links     []Link `json:"links"`
		Truncated bool   `json:"truncated"`
	}
	err = renderDocument(tabCtx, func(ctx context.Context) (*network.Response, error) {
		return chromedp.RunResponse(ctx, fetch.Enable(), chromedp.Navigate(rawURL))
	}, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(750*time.Millisecond),
			chromedp.Evaluate(`document.body ? document.body.innerText : ""`, &content),
			chromedp.Location(&finalURL),
			chromedp.Evaluate(`(() => { const anchors = Array.from(document.querySelectorAll('a[href]')).filter(a => a.getAttribute('href').trim() && !a.getAttribute('href').trim().startsWith('#')); return {links: anchors.slice(0,200).map(a => ({url:a.href,title:(a.innerText || a.title || '').slice(0,600)})),truncated:anchors.length>200}; })()`, &links),
		)
	})
	select {
	case violation := <-blocked:
		return nil, violation
	default:
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, &Error{Status: "timeout"}
		}
		return nil, &Error{Status: "web_unavailable"}
	}
	if err := b.network.validateURL(ctx, finalURL); err != nil {
		return nil, err
	}
	return &RenderedPage{URL: finalURL, Content: content, Links: links.Links, LinksTruncated: links.Truncated}, nil
}

// allocator 创建使用受控代理的浏览器执行环境及清理函数。
func (b *chromiumBrowser) allocator(ctx context.Context) (context.Context, context.CancelFunc, error) {
	executablePath, err := chromiumExecutablePath()
	if err != nil {
		return nil, nil, err
	}
	proxyURL, closeProxy, err := b.network.startProxy(ctx)
	if err != nil {
		return nil, nil, err
	}
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	options = append(options,
		chromedp.ExecPath(executablePath),
		chromedp.ProxyServer(proxyURL),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
		chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND, EXCLUDE 127.0.0.1"),
		chromedp.Flag("disable-quic", true),
		chromedp.Flag("force-webrtc-ip-handling-policy", "disable_non_proxied_udp"),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	allocatorCtx, cancel := chromedp.NewExecAllocator(ctx, options...)
	return allocatorCtx, func() { cancel(); closeProxy() }, nil
}

// chromiumExecutablePath 查找配置指定或系统可用的 Chromium 可执行文件。
func chromiumExecutablePath() (string, error) {
	home, _ := os.UserHomeDir()
	return findChromiumExecutable(os.Getenv("CHROME_BIN"), chromiumCandidates(runtime.GOOS, home), exec.LookPath)
}

// findChromiumExecutable 保持显式配置优先，自动探测只接受可执行文件。
func findChromiumExecutable(configured string, candidates []string, lookPath func(string) (string, error)) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		path, err := lookPath(configured)
		if err != nil {
			return "", fmt.Errorf("locate CHROME_BIN %q: %w", configured, err)
		}
		return path, nil
	}
	for _, name := range candidates {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("Chromium executable not found; install Chromium or set CHROME_BIN")
}

// handlePaused 校验并放行或阻断浏览器暂停的请求。
func (b *chromiumBrowser) handlePaused(ctx context.Context, event *fetch.EventRequestPaused, blocked chan<- error) {
	chromedpContext := chromedp.FromContext(ctx)
	if chromedpContext == nil || chromedpContext.Target == nil {
		return
	}
	ctx = cdp.WithExecutor(ctx, chromedpContext.Target)
	requestURL := event.Request.URL
	allowed := b.network.browserRequestAllowed(ctx, requestURL)
	if allowed {
		_ = fetch.ContinueRequest(event.RequestID).Do(ctx)
		return
	}
	_ = fetch.FailRequest(event.RequestID, network.ErrorReasonAborted).Do(ctx)
	if strings.HasPrefix(requestURL, "http:") || strings.HasPrefix(requestURL, "https:") {
		select {
		case blocked <- &Error{Status: "url_not_allowed"}:
		default:
		}
	}
}

// 校验 Chromium 渲染器满足浏览器接口。
var _ Browser = (*chromiumBrowser)(nil)

// renderDocument 仅在主文档响应成功时读取页面，避免把 HTTP 错误页当作正文。
func renderDocument(ctx context.Context, navigate func(context.Context) (*network.Response, error), extract func(context.Context) error) error {
	response, err := navigate(ctx)
	if err != nil {
		return err
	}
	if response == nil || response.Status < 200 || response.Status >= 300 {
		return &Error{Status: "web_unavailable"}
	}
	return extract(ctx)
}

// chromiumCandidates 优先查找 PATH，再查找当前系统的常见安装位置。
func chromiumCandidates(goos, home string) []string {
	candidates := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"}
	switch goos {
	case "darwin":
		roots := []string{"/Applications"}
		if home != "" {
			roots = append(roots, filepath.Join(home, "Applications"))
		}
		for _, root := range roots {
			candidates = append(candidates,
				filepath.Join(root, "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
				filepath.Join(root, "Chromium.app", "Contents", "MacOS", "Chromium"))
		}
	case "linux":
		candidates = append(candidates, "/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome", "/opt/google/chrome/chrome", "/snap/bin/chromium")
	}
	return candidates
}
