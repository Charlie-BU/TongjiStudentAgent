package webfetch

import (
	"context"
	"errors"
	"github.com/Charlie-BU/TongjiStudent/internal/platform/publicurl"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// maxRedirects 限制 HTTP 重定向检查的跳转链长度。
const maxRedirects = 5

// ipResolver 提供域名到 IP 地址的解析能力。
type ipResolver interface {
	// LookupNetIP 解析主机对应的 IP 地址集合。
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// connectionDialer 提供带上下文的网络连接能力。
type connectionDialer interface {
	// DialContext 建立受上下文控制的网络连接。
	DialContext(context.Context, string, string) (net.Conn, error)
}

// publicNetwork 统一管理公网解析、连接与请求校验。
type publicNetwork struct {
	resolver ipResolver
	dialer   connectionDialer
}

// newPublicNetwork 创建默认公网解析器与连接器。
func newPublicNetwork() *publicNetwork {
	return &publicNetwork{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}
}

// publicIP 判断 IP 是否符合允许访问的公网地址范围。
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!netip.MustParsePrefix("100.64.0.0/10").Contains(ip) && !netip.MustParsePrefix("2001:db8::/32").Contains(ip)
}

// resolve 解析主机并拒绝包含非公网地址的结果。
func (n *publicNetwork) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return nil, &Error{Status: "url_not_allowed"}
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	addresses, err := n.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, &Error{Status: "web_unavailable"}
	}
	for _, address := range addresses {
		if !publicIP(address) {
			return nil, &Error{Status: "url_not_allowed"}
		}
	}
	return addresses, nil
}

// validateURL 校验 URL 格式与目标主机的公网解析结果。
func (n *publicNetwork) validateURL(ctx context.Context, raw string) error {
	normalized, err := publicurl.PublicURL(raw)
	if err != nil {
		return &Error{Status: "url_not_allowed"}
	}
	u, _ := url.Parse(normalized)
	_, err = n.resolve(ctx, u.Hostname())
	return err
}

// dialContext 重新校验目标地址并直接连接已验证的公网 IP。
func (n *publicNetwork) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, &Error{Status: "web_unavailable"}
	}
	if port != "80" && port != "443" {
		return nil, &Error{Status: "url_not_allowed"}
	}
	addresses, err := n.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	var dialErr error
	for _, ip := range addresses {
		connection, err := n.dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
		dialErr = errors.Join(dialErr, err)
	}
	return nil, dialErr
}

// client 创建带公网连接策略与重定向校验的 HTTP 客户端。
func (n *publicNetwork) client(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // 使用本地公网校验后的解析结果。
	transport.DialContext = n.dialContext
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return n.validateURL(request.Context(), request.URL.String())
		},
	}
}

// browserSubresourceAllowed 判断浏览器资源协议是否允许进入后续校验。
func browserSubresourceAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme == "data" || u.Scheme == "blob" || u.Scheme == "about" {
		return true
	}
	return (u.Scheme == "http" || u.Scheme == "https") && !strings.EqualFold(u.Hostname(), "localhost")
}

// browserRequestAllowed 校验浏览器请求的协议、URL 与公网目标。
func (n *publicNetwork) browserRequestAllowed(ctx context.Context, raw string) bool {
	if !browserSubresourceAllowed(raw) {
		return false
	}
	u, _ := url.Parse(raw)
	if u.Scheme == "http" || u.Scheme == "https" {
		return n.validateURL(ctx, raw) == nil
	}
	return true
}
