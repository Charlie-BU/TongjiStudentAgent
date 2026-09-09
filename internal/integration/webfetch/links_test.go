package webfetch

import (
	"context"
	"errors"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"net/netip"
	"strings"
	"testing"
)

// TestStructuredLinks 验证链接补全、跳转解析、校验与输出边界。
func TestStructuredLinks(t *testing.T) {
	Convey("候选链接解析、校验、去重和边界", t, func() {
		n := fixtureNetwork()
		Convey("相对链接和 base 地址应还原并保留标题", func() {
			candidates, truncated := extractLinks([]byte(`<base href="/notices/"><a href="first#part">招生通知</a><a href="first">重复</a><a href="//other.example.org/next">下一页</a>`), "https://school.example.org/start")
			links, cut := n.validateLinks(context.Background(), candidates)
			So(truncated || cut, ShouldBeFalse)
			So(links, ShouldResemble, []Link{{URL: "https://school.example.org/notices/first", Title: "招生通知"}, {URL: "https://other.example.org/next", Title: "下一页"}})
		})
		Convey("DuckDuckGo 跳转应返回经过校验的目标", func() {
			links, _ := n.validateLinks(context.Background(), []Link{{URL: "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fschool.example.org%2Fnotice", Title: "通知"}, {URL: "https://duckduckgo.com/l/?uddg=http%3A%2F%2F127.0.0.1%2Fprivate"}})
			So(links, ShouldResemble, []Link{{URL: "https://school.example.org/notice", Title: "通知"}})
		})
		Convey("凭据、私有地址、协议及 DNS 私网拒绝", func() {
			n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
			})
			var candidates []Link
			for _, address := range []string{"file:///private", "javascript:alert(1)", "http://127.0.0.1", "https://user:pass@example.org", "https://example.org/?token=x", "https://example.org:8080", "https://private.example.org"} {
				candidates = append(candidates, Link{URL: address})
			}
			links, _ := n.validateLinks(context.Background(), candidates)
			So(links, ShouldBeEmpty)
		})
		Convey("DNS 每个结果仅校验一次但后续结果重新校验", func() {
			calls := 0
			n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				calls++
				return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
			})
			candidates := []Link{{URL: "https://example.org/1"}, {URL: "https://example.org/2"}}
			links, _ := n.validateLinks(context.Background(), candidates)
			So(links, ShouldHaveLength, 2)
			So(calls, ShouldEqual, 1)
			_, _ = n.validateLinks(context.Background(), candidates)
			So(calls, ShouldEqual, 2)
		})
		Convey("输出数量和标题长度受限", func() {
			var candidates []Link
			for i := 0; i < maxLinks+1; i++ {
				candidates = append(candidates, Link{URL: fmt.Sprintf("https://example.org/%d", i), Title: strings.Repeat("中", 400)})
			}
			links, cut := n.validateLinks(context.Background(), candidates)
			So(links, ShouldHaveLength, maxLinks)
			So(cut, ShouldBeTrue)
			So([]rune(links[0].Title), ShouldHaveLength, maxLinkTitleChars)
		})
		Convey("取消的校验不得继续解析", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			links, cut := n.validateLinks(ctx, []Link{{URL: "https://example.org"}})
			So(links, ShouldBeEmpty)
			So(cut, ShouldBeTrue)
		})
	})
}

// TestExtractLinkValidationCancellation 验证链接 DNS 校验中取消请求不会返回成功正文。
func TestExtractLinkValidationCancellation(t *testing.T) {
	Convey("父请求在链接校验中取消", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := NewClient(htmlClient(`<main><a href="https://linked.example.org/notice">notice</a></main>`), nil)
		client.network = fixtureNetwork()
		client.network.resolver = resolverFunc(func(_ context.Context, _ string, host string) ([]netip.Addr, error) {
			if host == "linked.example.org" {
				cancel()
				return nil, context.Canceled
			}
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		})
		result, err := client.Extract(ctx, ExtractInput{URL: "https://example.org"})
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		So(result, ShouldBeNil)
	})
	Convey("链接 DNS 自身超时不丢弃已取得的正文", t, func() {
		client := NewClient(htmlClient(`<main>usable content<a href="https://linked.example.org/notice">notice</a></main>`), nil)
		client.network = fixtureNetwork()
		client.network.resolver = resolverFunc(func(_ context.Context, _ string, host string) ([]netip.Addr, error) {
			if host == "linked.example.org" {
				return nil, context.DeadlineExceeded
			}
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		})
		result, err := client.Extract(context.Background(), ExtractInput{URL: "https://example.org"})
		So(err, ShouldBeNil)
		So(result, ShouldNotBeNil)
		So(result.Content, ShouldContainSubstring, "usable content")
		So(result.Links, ShouldBeEmpty)
	})
}
