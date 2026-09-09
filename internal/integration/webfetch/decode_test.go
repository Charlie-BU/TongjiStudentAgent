package webfetch

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	. "github.com/smartystreets/goconvey/convey"
)

// TestDecodePage 验证媒体类型、字符集与解码大小限制。
func TestDecodePage(t *testing.T) {
	Convey("媒体类型、字符集和解码后的大小边界", t, func() {
		for _, tc := range []struct{ name, body, kind, want string }{
			{"UTF8", "<main>中文</main>", "text/html; charset=utf-8", "中文"},
			{"GBK header", "<main>\xd6\xd0\xce\xc4</main>", "text/html; charset=gbk", "中文"},
			{"GBK meta", `<meta charset="gbk"><main>` + "\xd6\xd0\xce\xc4" + `</main>`, "text/html", "中文"},
			{"plain GBK", "\xd6\xd0\xce\xc4", "text/plain; charset=gbk", "中文"},
			{"BOM", "\xef\xbb\xbf中文", "text/plain", "中文"},
			{"XHTML", "<html><body>中文</body></html>", "application/xhtml+xml; charset=utf-8", "中文"},
			{"sniff HTML", "<html><body>中文</body></html>", "", "中文"},
			{"sniff plain", "plain text", "", "plain text"},
		} {
			Convey(tc.name, func() {
				decoded, _, err := decodePage([]byte(tc.body), tc.kind)
				So(err, ShouldBeNil)
				So(utf8.Valid(decoded), ShouldBeTrue)
				So(string(decoded), ShouldContainSubstring, tc.want)
			})
		}
		Convey("不支持的类型、错误声明和二进制拒绝", func() {
			for _, tc := range []struct{ body, kind string }{
				{"%PDF-1.7", "application/pdf"}, {"PK\x03\x04\x00", "application/zip"},
				{"\x89PNG\r\n\x1a\n", "image/png"}, {"binary", "application/octet-stream"},
				{"{}", "application/json"}, {"text", "text/html; charset="}, {"text", "text/plain; charset=unknown-charset"},
				{"%PDF-1.7", ""}, {"\x00\x01\x02", ""},
			} {
				browser := &fakeBrowser{page: &RenderedPage{URL: "https://example.org", Content: "must not render"}}
				result, err := responseClient(tc.body, tc.kind, browser).Extract(context.Background(), ExtractInput{URL: "https://example.org/file"})
				So(result, ShouldBeNil)
				So(Status(err), ShouldEqual, "fetch_failed")
				So(browser.calls, ShouldEqual, 0)
			}
		})
		Convey("转换成 UTF8 后仍受大小限制", func() {
			raw := []byte(strings.Repeat("\xd6\xd0", maxPageBytes/3+1))
			_, _, err := decodePage(raw, "text/plain; charset=gbk")
			So(Status(err), ShouldEqual, "fetch_failed")
		})
		Convey("链接标题也经过字符集解码", func() {
			client := responseClient("<main><a href='/notice'>\xd6\xd0\xce\xc4</a></main>", "text/html; charset=gbk", nil)
			client.network = fixtureNetwork()
			result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org"})
			So(err, ShouldBeNil)
			So(result.Links, ShouldHaveLength, 1)
			So(result.Links[0].Title, ShouldEqual, "中文")
		})
	})
}
