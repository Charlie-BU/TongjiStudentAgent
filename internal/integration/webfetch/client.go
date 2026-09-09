package webfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
)

const (
	// maxPageBytes 限制页面原始响应与解码后的字节数。
	maxPageBytes = 4 << 20
	// defaultHTTPTimeout 设置默认 HTTP 请求超时。
	defaultHTTPTimeout = 15 * time.Second
	// defaultBrowserTimeout 设置默认浏览器渲染与预检超时。
	defaultBrowserTimeout = 30 * time.Second
)

// Client 协调公开页面的 HTTP 提取、浏览器渲染与结果合并。
type Client struct {
	http    *http.Client
	browser Browser
	network *publicNetwork
}

// DisableBrowser 在开始处理请求前禁用客户端的浏览器提取能力。
func (c *Client) DisableBrowser() {
	if c != nil {
		c.browser = nil
	}
}

// NewFromEnv 装配默认公网 HTTP 客户端和 Chromium 渲染器。
func NewFromEnv() (*Client, error) {
	browser := NewChromiumBrowser(ChromiumConfig{Timeout: defaultBrowserTimeout})
	return NewClient(nil, browser), nil
}

// VerifyChromium 执行服务启动前的 Chromium 可用性检查。
func VerifyChromium(ctx context.Context) error {
	browser := newChromiumBrowser(ChromiumConfig{Timeout: defaultBrowserTimeout})
	if err := browser.Preflight(ctx); err != nil {
		return fmt.Errorf("Chromium preflight failed: %w", err)
	}
	return nil
}

// NewClient 创建提取客户端并支持注入 HTTP 客户端和浏览器。
func NewClient(httpClient *http.Client, browser Browser) *Client {
	var network *publicNetwork
	if httpClient == nil {
		network = newPublicNetwork()
		httpClient = network.client(defaultHTTPTimeout)
	}
	return &Client{http: httpClient, browser: browser, network: network}
}

// Extract 提取公开页面的正文和候选链接并汇总可用来源。
func (c *Client) Extract(ctx context.Context, input ExtractInput) (result *ExtractResponse, err error) {
	started := time.Now()
	source := "none"
	defer func() {
		if err == nil && result != nil && len(result.Links) > 0 {
			network := c.network
			if network == nil {
				network = newPublicNetwork()
			}
			links, truncated := network.validateLinks(ctx, result.Links)
			result.Links, result.LinksTruncated = links, result.LinksTruncated || truncated
		}
		if ctx.Err() != nil {
			result, err = nil, ctx.Err()
		}
		status := "ok"
		if err != nil {
			status = Status(err)
		}
		logging.CtxInfo(ctx, "webfetch source=%s status=%s duration_ms=%d", source, status, time.Since(started).Milliseconds())
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.http == nil {
		return nil, &Error{Status: "web_unavailable"}
	}
	if c.network != nil {
		if err := c.network.validateURL(ctx, input.URL); err != nil {
			return nil, err
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, input.URL, nil)
	if err != nil {
		return nil, &Error{Status: "url_not_allowed"}
	}
	request.Header.Set("User-Agent", "TongjiStudentAgent/1.0 public-page-reader")
	request.Header.Set("Accept", "text/html, text/plain;q=0.9, application/xhtml+xml;q=0.8")
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var stable *Error
		if errors.As(err, &stable) && stable.Status == "url_not_allowed" {
			return nil, stable
		}
		if c.browser != nil {
			return c.render(ctx, input.URL, &source)
		}
		if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
			return nil, &Error{Status: "timeout"}
		}
		return nil, &Error{Status: "web_unavailable"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if c.browser != nil {
			return c.render(ctx, input.URL, &source)
		}
		return nil, &Error{Status: "fetch_failed"}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes {
		return nil, &Error{Status: "fetch_failed"}
	}
	finalURL := input.URL
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	decoded, mediaType, decodeErr := decodePage(data, response.Header.Get("Content-Type"))
	if decodeErr != nil {
		return nil, decodeErr
	}
	data = decoded
	if mediaType == "text/plain" {
		content := normalizeText(string(data))
		if content == "" {
			return nil, &Error{Status: "fetch_failed"}
		}
		source = "http_text"
		return &ExtractResponse{URL: finalURL, Content: content, Source: source}, nil
	}
	candidates := extractCandidates(data)
	staticContent, staticSources := mergeCandidates(candidates)
	links, linksTruncated := extractLinks(data, finalURL)
	if c.browser == nil {
		if staticContent == "" {
			return nil, &Error{Status: "fetch_failed"}
		}
		source = staticSources
		return &ExtractResponse{URL: finalURL, Content: staticContent, Source: source, Links: links, LinksTruncated: linksTruncated}, nil
	}

	// 合并静态与动态正文，并在浏览器失败时保留可用静态结果。
	rendered, renderErr := c.render(ctx, finalURL, &source)
	if renderErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if staticContent != "" {
			source = staticSources
			return &ExtractResponse{URL: finalURL, Content: staticContent, Source: source, Links: links, LinksTruncated: linksTruncated}, nil
		}
		return nil, renderErr
	}
	content := mergeText(staticContent, rendered.Content)
	if content == "" {
		return nil, &Error{Status: "fetch_failed"}
	}
	if staticSources != "" {
		source = staticSources + "+browser"
	}
	return &ExtractResponse{URL: rendered.URL, Content: content, Source: source, Links: append(links, rendered.Links...), LinksTruncated: linksTruncated || rendered.LinksTruncated}, nil
}

// render 调用浏览器并将渲染结果转换为统一提取响应。
func (c *Client) render(ctx context.Context, address string, source *string) (*ExtractResponse, error) {
	rendered, err := c.browser.Render(ctx, address)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var stable *Error
		if errors.As(err, &stable) {
			return nil, stable
		}
		return nil, &Error{Status: "web_unavailable"}
	}
	if rendered == nil || strings.TrimSpace(rendered.Content) == "" {
		return nil, &Error{Status: "fetch_failed"}
	}
	*source = "browser"
	return &ExtractResponse{URL: rendered.URL, Content: normalizeText(rendered.Content), Source: *source, Links: rendered.Links, LinksTruncated: rendered.LinksTruncated}, nil
}
