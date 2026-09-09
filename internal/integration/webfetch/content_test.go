package webfetch

import (
	"context"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestContentPreservesRepeatedFacts 验证重复事实在所属正文上下文中保留。
func TestContentPreservesRepeatedFacts(t *testing.T) {
	Convey("正文重复事实属于各自上下文", t, func() {
		for _, markup := range []string{
			`<main><p>本科生</p><p>截止日期：9月10日</p><p>研究生</p><p>截止日期：9月10日</p></main>`,
			`<main><table><tr><td>本科生</td><td>截止日期：9月10日</td></tr><tr><td>研究生</td><td>截止日期：9月10日</td></tr></table></main>`,
		} {
			text, _ := mergeCandidates(extractCandidates([]byte(markup)))
			So(text, ShouldEqual, "本科生\n截止日期：9月10日\n研究生\n截止日期：9月10日")
		}
		So(normalizeText("100\n100"), ShouldEqual, "100\n100")
		So(mergeText("甲\n相同\n乙\n相同", "甲\n相同\n乙\n相同"), ShouldEqual, "甲\n相同\n乙\n相同")
		So(mergeText("甲\n相同", "乙\n相同"), ShouldEqual, "甲\n相同\n乙\n相同")
	})
}

// responseClient 创建返回指定响应内容与媒体类型的测试提取客户端。
func responseClient(body, contentType string, browser Browser) *Client {
	return NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}, browser)
}

// TestExtractOverlappingRepresentationsWithinBudget 验证重叠正文合并后仍能保留动态补充内容。
func TestExtractOverlappingRepresentationsWithinBudget(t *testing.T) {
	Convey("unique page should fit the maximum content budget", t, func() {
		body := strings.Repeat("正文", 3500)
		browser := &fakeBrowser{page: &RenderedPage{URL: "https://example.org", Content: "导航\n" + body + "\n动态截止日期：9月10日"}}
		result, err := responseClient("<body><nav>导航</nav><main>"+body+"</main></body>", "text/html", browser).Extract(context.Background(), ExtractInput{URL: "https://example.org"})
		So(err, ShouldBeNil)
		chars := []rune(result.Content)
		t.Logf("unique_chars=%d merged_chars=%d", len([]rune(browser.page.Content)), len(chars))
		if len(chars) > 16000 {
			chars = chars[:16000]
		}
		So(strings.Contains(string(chars), "动态截止日期"), ShouldBeTrue)
	})
}

// TestExtractDeclaredCharset 验证提取前正确解码声明的字符集。
func TestExtractDeclaredCharset(t *testing.T) {
	Convey("declared GBK content must be decoded before HTML extraction", t, func() {
		result, err := responseClient("<main>\xd6\xd0\xce\xc4</main>", "text/html; charset=gbk", &fakeBrowser{err: &Error{Status: "web_unavailable"}}).Extract(context.Background(), ExtractInput{URL: "https://example.org"})
		So(err, ShouldBeNil)
		t.Logf("content=%q", result.Content)
		So(result.Content, ShouldEqual, "中文")
	})
}

// TestExtractRejectsBinaryResponse 验证二进制响应不会被当成正文返回。
func TestExtractRejectsBinaryResponse(t *testing.T) {
	Convey("unsupported PDF must not return its file syntax as page text", t, func() {
		result, err := responseClient("%PDF-1.7\n1 0 obj\n<</Type /Catalog>>\nendobj", "application/pdf", &fakeBrowser{err: &Error{Status: "web_unavailable"}}).Extract(context.Background(), ExtractInput{URL: "https://example.org/file.pdf"})
		if result != nil {
			t.Logf("source=%s content=%q", result.Source, result.Content)
		}
		So(err, ShouldNotBeNil)
	})
}

// TestMergeParagraphOverlap 验证段落重叠合并与重复事实保留。
func TestMergeParagraphOverlap(t *testing.T) {
	Convey("按上下文合并表示且保留重复事实", t, func() {
		So(mergeText("标题\n第一段\n第二段", "第一段\n第二段\n新段落"), ShouldEqual, "标题\n第一段\n第二段\n新段落")
		So(mergeText("本科生\n100", "研究生\n100"), ShouldEqual, "本科生\n100\n研究生\n100")
		So(mergeText("标题\n100\n100", "导航\n标题\n100\n100\n补充"), ShouldEqual, "导航\n标题\n100\n100\n补充")
		So(mergeText("第十条", "第十条附则"), ShouldEqual, "第十条\n第十条附则")
		So(mergeText("A\nB\nA\nB", "A\nB\nC"), ShouldEqual, "A\nB\nA\nB\nC")
	})
}
