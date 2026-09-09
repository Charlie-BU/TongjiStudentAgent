package webfetch

import (
	"context"
	"errors"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestNewFromEnv 验证默认客户端装配且不启动浏览器进程。
func TestNewFromEnv(t *testing.T) {
	Convey("默认客户端装配且不启动外部进程", t, func() {
		client, err := NewFromEnv()
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)
		So(client.http, ShouldNotBeNil)
		So(client.browser, ShouldNotBeNil)
		browser, ok := client.browser.(*chromiumBrowser)
		So(ok, ShouldBeTrue)
		So(browser.config.Timeout, ShouldEqual, defaultBrowserTimeout)
	})
}

// roundTripFunc 将函数适配为测试用 HTTP 传输层。
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip 调用测试函数生成 HTTP 响应。
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

// htmlClient 创建返回固定 HTML 文档的测试客户端。
func htmlClient(document string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(document)),
			Request:    request,
		}, nil
	})}
}

// fakeBrowser 记录浏览器调用次数并提供预设结果。
type fakeBrowser struct {
	calls int
	page  *RenderedPage
	err   error
}

// Render 记录调用并返回预设浏览器结果。
func (f *fakeBrowser) Render(_ context.Context, _ string) (*RenderedPage, error) {
	f.calls++
	return f.page, f.err
}

// TestHTTPRepresentations 验证静态正文与浏览器补充内容共同返回。
func TestHTTPRepresentations(t *testing.T) {
	Convey("HTTP 正文与浏览器补充", t, func() {
		full := "页面标题\n第一章节\n第一条完整说明\n第二条完整说明\n第二章节\n第三条完整说明\n第四条完整说明\n第三章节\n第五条完整说明\n第六条完整说明\n第七条完整说明\n第八条完整说明"
		tests := []struct{ name, html, source string }{
			{"noscript", `<body>Enable JavaScript<noscript><main>` + full + `</main></noscript></body>`, "http_noscript"},
			{"hydration", `<body>Enable JavaScript<script type="application/json">{"page":"<main>` + strings.ReplaceAll(full, "\n", `\n`) + `</main>"}</script></body>`, "http_hydration_json"},
			{"static", `<body><main>` + full + `</main></body>`, "http_main"},
		}
		for _, test := range tests {
			Convey(test.name, func() {
				browser := &fakeBrowser{page: &RenderedPage{URL: "https://example.org/page", Content: "动态补充"}}
				response, err := NewClient(htmlClient(test.html), browser).Extract(context.Background(), ExtractInput{URL: "https://example.org/page"})
				So(err, ShouldBeNil)
				So(response.Source, ShouldEqual, test.source+"+http_body+browser")
				So(response.Content, ShouldContainSubstring, "第八条完整说明")
				So(response.Content, ShouldContainSubstring, "动态补充")
				So(browser.calls, ShouldEqual, 1)
			})
		}
	})
}

// TestExtractLinksAcrossRepresentations 验证静态、渲染及降级路径的链接输出。
func TestExtractLinksAcrossRepresentations(t *testing.T) {
	Convey("链接在静态、动态和降级路径一致输出", t, func() {
		for _, mode := range []string{"static", "merged", "failed", "browser"} {
			Convey(mode, func() {
				var browser Browser
				page := `<main><a href="/notice#part">招生通知</a></main>`
				if mode != "static" {
					fake := &fakeBrowser{page: &RenderedPage{URL: "https://example.org/final", Content: "动态章节", Links: []Link{{URL: "https://example.org/notice", Title: "重复"}, {URL: "https://example.org/dynamic", Title: "动态来源"}, {URL: "http://127.0.0.1/private", Title: "拒绝"}}}}
					if mode == "failed" {
						fake.err = &Error{Status: "web_unavailable"}
					}
					if mode == "browser" {
						page = `<body></body>`
					}
					browser = fake
				}
				client := NewClient(htmlClient(page), browser)
				client.network = fixtureNetwork()
				result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org/page"})
				So(err, ShouldBeNil)
				So(result.LinksTruncated, ShouldBeFalse)
				So(result.Links[0].URL, ShouldEqual, "https://example.org/notice")
				if mode == "merged" || mode == "browser" {
					So(result.Links, ShouldHaveLength, 2)
					So(result.Content, ShouldContainSubstring, "动态章节")
				} else {
					So(result.Links, ShouldHaveLength, 1)
				}
				if mode != "browser" {
					So(result.Content, ShouldContainSubstring, "招生通知")
					So(result.Links[0].Title, ShouldEqual, "招生通知")
				}
			})
		}
	})
}

// browserFunc 将函数适配为测试用浏览器。
type browserFunc func(context.Context, string) (*RenderedPage, error)

// Render 调用测试函数返回页面渲染结果。
func (f browserFunc) Render(ctx context.Context, address string) (*RenderedPage, error) {
	return f(ctx, address)
}

// trackedBody 记录测试响应体的关闭状态。
type trackedBody struct {
	io.Reader
	closed bool
}

// Close 标记测试响应体已关闭。
func (b *trackedBody) Close() error { b.closed = true; return nil }

// failedReader 模拟响应体读取失败。
type failedReader struct{}

// Read 返回预设的读取错误。
func (failedReader) Read([]byte) (int, error) { return 0, errors.New("fixture read failure") }

// TestExtractFailureAndBounds 验证取消、响应边界、资源关闭与错误降级。
func TestExtractFailureAndBounds(t *testing.T) {
	Convey("取消、响应边界、资源关闭和网络错误", t, func() {
		Convey("取消发生在浏览器阶段仍必须传递", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			browser := browserFunc(func(context.Context, string) (*RenderedPage, error) { cancel(); return nil, context.Canceled })
			result, err := NewClient(htmlClient(`<main>静态正文</main>`), browser).Extract(ctx, ExtractInput{URL: "https://example.org"})
			So(result, ShouldBeNil)
			So(errors.Is(err, context.Canceled), ShouldBeTrue)
		})
		Convey("响应过大或读取失败应关闭 Body", func() {
			for _, reader := range []io.Reader{strings.NewReader(strings.Repeat("x", maxPageBytes+1)), failedReader{}} {
				body := &trackedBody{Reader: reader}
				client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
				})}, nil)
				result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org"})
				So(result, ShouldBeNil)
				So(Status(err), ShouldEqual, "fetch_failed")
				So(body.closed, ShouldBeTrue)
			}
		})
		Convey("纯文本保留重复值并关闭 Body", func() {
			body := &trackedBody{Reader: strings.NewReader("100\n100")}
			client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: body}, nil
			})}, nil)
			result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org"})
			So(err, ShouldBeNil)
			So(result.Content, ShouldEqual, "100\n100")
			So(result.Source, ShouldEqual, "http_text")
			So(body.closed, ShouldBeTrue)
		})
		Convey("传输错误和非 2xx 可降级，安全拒绝不得降级", func() {
			for _, mode := range []string{"error", "non2xx", "rejected"} {
				browser := &fakeBrowser{page: &RenderedPage{URL: "https://example.org", Content: "浏览器正文"}}
				client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					if mode == "rejected" {
						return nil, &Error{Status: "url_not_allowed"}
					}
					if mode == "error" {
						return nil, errors.New("fixture network error")
					}
					return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
				})}, browser)
				result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org"})
				if mode == "rejected" {
					So(Status(err), ShouldEqual, "url_not_allowed")
					So(browser.calls, ShouldEqual, 0)
				} else {
					So(err, ShouldBeNil)
					So(result.Source, ShouldEqual, "browser")
				}
			}
		})
	})
}

// TestPublicIP 验证公网与禁止访问 IP 的分类。
func TestPublicIP(t *testing.T) {
	Convey("IP 公网分类", t, func() {
		for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1", "2001:db8::1"} {
			parsed, err := parseIP(address)
			So(err, ShouldBeNil)
			So(publicIP(parsed), ShouldBeFalse)
		}
		parsed, err := parseIP("1.1.1.1")
		So(err, ShouldBeNil)
		So(publicIP(parsed), ShouldBeTrue)
	})
}
