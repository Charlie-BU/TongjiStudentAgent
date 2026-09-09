package webfetch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

// TestProxyRejectsPrivateDestinations 验证代理拒绝私网目标与 DNS 重绑定。
func TestProxyRejectsPrivateDestinations(t *testing.T) {
	Convey("代理 HTTP 和 CONNECT 均拒绝私网和重绑定", t, func() {
		for _, method := range []string{http.MethodGet, http.MethodConnect} {
			for _, rebound := range []bool{false, true} {
				n := fixtureNetwork()
				calls := 0
				n.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
					calls++
					if rebound && calls == 1 {
						return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
					}
					return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
				})
				dialed := false
				n.dialer = dialFunc(func(context.Context, string, string) (net.Conn, error) {
					dialed = true
					return nil, fmt.Errorf("unexpected dial")
				})
				transport := n.client(time.Second).Transport.(*http.Transport)
				defer transport.CloseIdleConnections()
				proxy := &browserProxy{network: n, transport: transport}
				request := httptest.NewRequest(method, "http://example.org/page", nil)
				if method == http.MethodConnect {
					request.Host = "example.org:443"
				}
				result := httptest.NewRecorder()
				proxy.ServeHTTP(result, request)
				So(result.Code, ShouldNotEqual, http.StatusOK)
				So(dialed, ShouldBeFalse)
				if rebound {
					So(calls, ShouldEqual, 2)
				}
			}
		}
	})
}

// TestProxyHTTPPinsAndStripsHeaders 验证代理连接公网 IP 并移除逐跳及代理凭据头。
func TestProxyHTTPPinsAndStripsHeaders(t *testing.T) {
	Convey("HTTP 转发至已校验 IP 且不转发代理凭据", t, func() {
		n := fixtureNetwork()
		observed := make(chan *http.Request, 1)
		targets := make(chan string, 1)
		n.dialer = dialFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			targets <- address
			client, server := net.Pipe()
			go func() {
				defer server.Close()
				r, err := http.ReadRequest(bufio.NewReader(server))
				if err != nil {
					return
				}
				observed <- r
				_, _ = io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 7\r\nConnection: close\r\n\r\nfixture")
			}()
			return client, nil
		})
		transport := n.client(time.Second).Transport.(*http.Transport)
		defer transport.CloseIdleConnections()
		request := httptest.NewRequest(http.MethodGet, "http://example.org/page", nil)
		request.Header.Set("Proxy-Authorization", "fixture-secret")
		request.Header.Set("Connection", "X-Hop")
		request.Header.Set("X-Hop", "remove")
		recorder := httptest.NewRecorder()
		(&browserProxy{network: n, transport: transport}).ServeHTTP(recorder, request)
		So(recorder.Code, ShouldEqual, 200)
		So(recorder.Body.String(), ShouldEqual, "fixture")
		So(<-targets, ShouldEqual, "1.1.1.1:80")
		received := <-observed
		defer received.Body.Close()
		So(received.Host, ShouldEqual, "example.org")
		So(received.Header.Get("Proxy-Authorization"), ShouldBeEmpty)
		So(received.Header.Get("X-Hop"), ShouldBeEmpty)
	})
}

// TestProxyTunnelLifecycle 验证 CONNECT 隧道的数据转发与关闭行为。
func TestProxyTunnelLifecycle(t *testing.T) {
	Convey("CONNECT 绑定公网 IP 并在渲染结束时关闭隧道", t, func() {
		n := fixtureNetwork()
		targets := make(chan string, 1)
		upstreamClosed := make(chan struct{})
		n.dialer = dialFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			targets <- address
			client, server := net.Pipe()
			go func() { defer close(upstreamClosed); defer server.Close(); _, _ = io.Copy(server, server) }()
			return client, nil
		})
		proxyURL, closeProxy, err := n.startProxy(context.Background())
		So(err, ShouldBeNil)
		defer closeProxy()
		parsed, _ := url.Parse(proxyURL)
		connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
		So(err, ShouldBeNil)
		defer connection.Close()
		So(connection.SetDeadline(time.Now().Add(3*time.Second)), ShouldBeNil)
		// 在同一次写入中覆盖 CONNECT 头部后已缓冲的隧道数据。
		_, err = io.WriteString(connection, "CONNECT example.org:443 HTTP/1.1\r\nHost: example.org:443\r\n\r\nping")
		So(err, ShouldBeNil)
		reader := bufio.NewReader(connection)
		response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
		So(err, ShouldBeNil)
		So(response.StatusCode, ShouldEqual, 200)
		So(<-targets, ShouldEqual, "1.1.1.1:443")
		payload := make([]byte, 4)
		_, err = io.ReadFull(reader, payload)
		So(err, ShouldBeNil)
		So(string(payload), ShouldEqual, "ping")
		closeProxy()
		_, err = reader.ReadByte()
		So(err, ShouldNotBeNil)
		select {
		case <-upstreamClosed:
		case <-time.After(3 * time.Second):
			t.Fatal("tunnel upstream not closed")
		}
	})
}

// TestProxyInvalidConnectAndHTTPURLs 验证非法隧道目标与 HTTP 请求被拒绝。
func TestProxyInvalidConnectAndHTTPURLs(t *testing.T) {
	Convey("非法隧道目标和请求协议不得连接", t, func() {
		n := fixtureNetwork()
		transport := n.client(time.Second).Transport.(*http.Transport)
		defer transport.CloseIdleConnections()
		p := &browserProxy{network: n, transport: transport}
		for _, host := range []string{"localhost:443", "127.0.0.1:443", "example.org:22", "user@example.org:443", "example.org"} {
			request := httptest.NewRequest(http.MethodConnect, "http://example.org", nil)
			request.Host = host
			recorder := httptest.NewRecorder()
			p.ServeHTTP(recorder, request)
			So(recorder.Code, ShouldEqual, 403)
		}
		for _, target := range []string{"https://example.org", "http://127.0.0.1", "http://example.org/?password=x"} {
			recorder := httptest.NewRecorder()
			p.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, strings.NewReader("")))
			So(recorder.Code, ShouldEqual, 403)
		}
	})
}
