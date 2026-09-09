package webfetch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// parseIP 解析测试使用的 IP 字面量。
func parseIP(value string) (netip.Addr, error) { return netip.ParseAddr(value) }

// resolverFunc 将函数适配为测试用域名解析器。
type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

// LookupNetIP 调用测试函数返回主机解析结果。
func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

// dialFunc 将函数适配为测试用网络连接器。
type dialFunc func(context.Context, string, string) (net.Conn, error)

// DialContext 调用测试函数模拟网络连接。
func (f dialFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

// fixtureNetwork 创建固定公网解析且默认拒绝实际拨号的测试网络。
func fixtureNetwork() *publicNetwork {
	return &publicNetwork{
		resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		}),
		dialer: dialFunc(func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("unexpected dial") }),
	}
}

// TestPublicNetworkValidation 验证 URL、DNS 解析与公网地址限制。
func TestPublicNetworkValidation(t *testing.T) {
	Convey("公网网络正常、拒绝与解析失败分支", t, func() {
		n := fixtureNetwork()
		for _, raw := range []string{"http://127.0.0.1", "http://[::1]", "http://10.0.0.1", "http://169.254.169.254", "http://100.64.0.1", "http://2130706433", "http://localhost", "http://foo.local", "http://example.org:8080", "file:///etc/passwd", "https://user:pass@example.org", "https://example.org/?token=x"} {
			So(Status(n.validateURL(context.Background(), raw)), ShouldEqual, "url_not_allowed")
		}
		So(n.validateURL(context.Background(), "https://example.org/page"), ShouldBeNil)
		Convey("混合 DNS 中任意私网地址都拒绝", func() {
			n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.1")}, nil
			})
			So(Status(n.validateURL(context.Background(), "https://example.org")), ShouldEqual, "url_not_allowed")
		})
		Convey("解析失败与空答案不可放行", func() {
			for _, err := range []error{nil, errors.New("resolver unavailable")} {
				n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) { return nil, err })
				So(Status(n.validateURL(context.Background(), "https://example.org")), ShouldEqual, "web_unavailable")
			}
		})
	})
}

// TestDialPinsValidatedIP 验证连接阶段重新校验并绑定公网 IP。
func TestDialPinsValidatedIP(t *testing.T) {
	Convey("连接阶段重新校验并拨号至 IP 字面量", t, func() {
		n := fixtureNetwork()
		calls := 0
		n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
			calls++
			if calls == 1 {
				return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		})
		Convey("初检通过后 DNS 重绑定不得发起连接", func() {
			So(n.validateURL(context.Background(), "http://example.org"), ShouldBeNil)
			dialed := false
			n.dialer = dialFunc(func(context.Context, string, string) (net.Conn, error) {
				dialed = true
				return nil, errors.New("unexpected")
			})
			_, err := n.dialContext(context.Background(), "tcp", "example.org:80")
			So(Status(err), ShouldEqual, "url_not_allowed")
			So(dialed, ShouldBeFalse)
		})
		Convey("公网连接只能使用已解析 IP", func() {
			var address string
			n.dialer = dialFunc(func(_ context.Context, _ string, target string) (net.Conn, error) {
				address = target
				return nil, errors.New("fixture")
			})
			_, err := n.dialContext(context.Background(), "tcp", "example.org:443")
			So(err, ShouldNotBeNil)
			So(address, ShouldEqual, "1.1.1.1:443")
			So(calls, ShouldEqual, 1)
		})
		Convey("非法端口不得拨号", func() {
			_, err := n.dialContext(context.Background(), "tcp", "example.org:22")
			So(Status(err), ShouldEqual, "url_not_allowed")
			So(calls, ShouldEqual, 0)
		})
	})
}

// TestPublicHTTPRedirects 验证 HTTP 重定向的安全边界与次数限制。
func TestPublicHTTPRedirects(t *testing.T) {
	Convey("真实 HTTP Transport 使用离线管道验证重定向边界", t, func() {
		for _, location := range []string{"http://127.0.0.1/private", "https://example.org/?token=private", "http://example.org/loop"} {
			n := fixtureNetwork()
			calls := 0
			n.dialer = dialFunc(func(context.Context, string, string) (net.Conn, error) {
				calls++
				client, server := net.Pipe()
				go func() {
					defer server.Close()
					request, err := http.ReadRequest(bufio.NewReader(server))
					if err != nil {
						return
					}
					_ = request.Body.Close()
					_, _ = fmt.Fprintf(server, "HTTP/1.1 302 Found\r\nLocation: %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", location)
				}()
				return client, nil
			})
			client := n.client(defaultHTTPTimeout)
			t.Cleanup(client.CloseIdleConnections)
			t.Setenv("HTTP_PROXY", "http://proxy.example.org:80")
			So(client.Transport.(*http.Transport).Proxy, ShouldBeNil)
			response, err := client.Get("http://example.org/start")
			if response != nil {
				_ = response.Body.Close()
			}
			So(err, ShouldNotBeNil)
			if location == "http://example.org/loop" {
				So(calls, ShouldEqual, maxRedirects)
			} else {
				So(Status(err), ShouldEqual, "url_not_allowed")
				So(calls, ShouldEqual, 1)
			}
		}
	})
}
