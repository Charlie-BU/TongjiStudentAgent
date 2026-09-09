package webfetch

import (
	"context"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/network"
	. "github.com/smartystreets/goconvey/convey"
	"net/netip"
	"strings"
	"testing"
)

// TestBrowserRequestGate 验证浏览器主文档与子资源的公开网络限制。
func TestBrowserRequestGate(t *testing.T) {
	Convey("主文档与子资源采用相同的公开网络拒绝规则", t, func() {
		n := fixtureNetwork()
		for _, address := range []string{"https://example.org/main", "https://cdn.example.org/script.js", "data:text/plain,fixture", "blob:https://example.org/id", "about:blank"} {
			So(n.browserRequestAllowed(context.Background(), address), ShouldBeTrue)
		}
		for _, address := range []string{"file:///private", "ftp://example.org/file", "http://127.0.0.1", "https://example.org:8443", "https://example.org/?token=x", "ws://example.org"} {
			So(n.browserRequestAllowed(context.Background(), address), ShouldBeFalse)
		}
		n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("192.168.1.1")}, nil
		})
		So(n.browserRequestAllowed(context.Background(), "https://cdn.example.org/script.js"), ShouldBeFalse)
	})
}

// TestChromiumExecutableMissing 验证无效浏览器路径会阻止执行环境创建。
func TestChromiumExecutableMissing(t *testing.T) {
	Convey("配置无效的浏览器路径返回启动错误且不执行进程", t, func() {
		t.Setenv("CHROME_BIN", t.TempDir()+"/missing-chromium")
		_, err := chromiumExecutablePath()
		So(err, ShouldNotBeNil)
		_, _, err = newChromiumBrowser(ChromiumConfig{}).allocator(context.Background())
		So(err, ShouldNotBeNil)
	})
}

// TestRenderDocumentStatus 验证主文档响应检查先于正文读取，不启动真实浏览器。
func TestRenderDocumentStatus(t *testing.T) {
	Convey("仅成功的主文档响应可进入正文提取", t, func() {
		for _, status := range []int64{200, 204, 299, 301, 403, 404, 500} {
			Convey(fmt.Sprintf("HTTP %d", status), func() {
				extracted := false
				err := renderDocument(context.Background(), func(context.Context) (*network.Response, error) {
					return &network.Response{Status: status}, nil
				}, func(context.Context) error { extracted = true; return nil })
				So(extracted, ShouldEqual, status >= 200 && status < 300)
				if extracted {
					So(err, ShouldBeNil)
				} else {
					So(Status(err), ShouldEqual, "web_unavailable")
				}
			})
		}
		Convey("缺失响应不得读取正文", func() {
			called := false
			err := renderDocument(context.Background(), func(context.Context) (*network.Response, error) { return nil, nil }, func(context.Context) error { called = true; return nil })
			So(Status(err), ShouldEqual, "web_unavailable")
			So(called, ShouldBeFalse)
		})
		Convey("导航失败保留原错误且不读取正文", func() {
			called := false
			err := renderDocument(context.Background(), func(context.Context) (*network.Response, error) { return nil, context.Canceled }, func(context.Context) error { called = true; return nil })
			So(errors.Is(err, context.Canceled), ShouldBeTrue)
			So(called, ShouldBeFalse)
		})
		Convey("正文读取失败向上传递", func() {
			err := renderDocument(context.Background(), func(context.Context) (*network.Response, error) { return &network.Response{Status: 200}, nil }, func(context.Context) error { return context.DeadlineExceeded })
			So(errors.Is(err, context.DeadlineExceeded), ShouldBeTrue)
		})
	})
}

// TestFindChromiumExecutable 用模拟文件查找验证优先级，不依赖宿主机浏览器。
func TestFindChromiumExecutable(t *testing.T) {
	Convey("自动发现浏览器", t, func() {
		for _, tc := range []struct{ name, configured, platform, home, available, want string }{
			{"显式配置优先", " /custom/chrome ", "darwin", "/users/test", "/custom/chrome", "/custom/chrome"},
			{"PATH 优先", "", "darwin", "/users/test", "chromium", "chromium"},
			{"macOS 系统 Chrome", "", "darwin", "/users/test", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
			{"macOS 用户 Chromium", "", "darwin", "/users/test", "/users/test/Applications/Chromium.app/Contents/MacOS/Chromium", "/users/test/Applications/Chromium.app/Contents/MacOS/Chromium"},
			{"Linux 固定路径", "", "linux", "", "/opt/google/chrome/chrome", "/opt/google/chrome/chrome"},
			{"显式配置错误不静默回退", "/missing", "darwin", "", "chromium", ""},
			{"未安装浏览器", "", "darwin", "", "", ""},
		} {
			Convey(tc.name, func() {
				var visited []string
				path, err := findChromiumExecutable(tc.configured, chromiumCandidates(tc.platform, tc.home), func(candidate string) (string, error) {
					visited = append(visited, candidate)
					if candidate == tc.available {
						return candidate, nil
					}
					return "", errors.New("not executable")
				})
				So(path, ShouldEqual, tc.want)
				if tc.want == "" {
					So(err, ShouldNotBeNil)
				} else {
					So(err, ShouldBeNil)
				}
				if tc.configured != "" {
					So(visited, ShouldResemble, []string{strings.TrimSpace(tc.configured)})
				}
				if tc.available == "chromium" && tc.configured == "" {
					So(visited, ShouldResemble, []string{"chromium"})
				}
			})
		}
	})
}
