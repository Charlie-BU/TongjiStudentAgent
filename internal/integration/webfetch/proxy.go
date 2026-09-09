package webfetch

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// browserProxy 转发浏览器的 HTTP 请求与 HTTPS 隧道连接。
type browserProxy struct {
	network   *publicNetwork
	transport *http.Transport
}

// startProxy 启动浏览器本地出口代理并返回地址及清理函数。
func (n *publicNetwork) startProxy(parent context.Context) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	transport := n.client(defaultHTTPTimeout).Transport.(*http.Transport)
	proxy := &browserProxy{network: n, transport: transport}
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() { _ = server.Serve(listener) }()
	var once sync.Once
	closeProxy := func() { once.Do(func() { cancel(); _ = server.Close(); transport.CloseIdleConnections() }) }
	stop := context.AfterFunc(ctx, closeProxy)
	return "http://" + listener.Addr().String(), func() { stop(); closeProxy() }, nil
}

// ServeHTTP 处理代理 HTTP 请求或分派 CONNECT 隧道请求。
func (p *browserProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.connect(w, r)
		return
	}
	if r.URL.Scheme != "http" || p.network.validateURL(r.Context(), r.URL.String()) != nil {
		http.Error(w, "url_not_allowed", http.StatusForbidden)
		return
	}
	request := r.Clone(r.Context())
	request.RequestURI = ""
	request.Host = request.URL.Host
	stripHopHeaders(request.Header)
	response, err := p.transport.RoundTrip(request)
	if err != nil {
		http.Error(w, "web_unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	stripHopHeaders(response.Header)
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

// connect 建立经过公网校验且随上下文关闭的双向隧道。
func (p *browserProxy) connect(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || host == "" || (port != "80" && port != "443") || p.network.validateURL(r.Context(), "https://"+r.Host) != nil {
		http.Error(w, "url_not_allowed", http.StatusForbidden)
		return
	}
	upstream, err := p.network.dialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "url_not_allowed", http.StatusForbidden)
		return
	}
	defer upstream.Close()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "web_unavailable", http.StatusInternalServerError)
		return
	}
	downstream, buffer, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer downstream.Close()
	// 将已接管连接绑定到渲染上下文，并等待双向转发结束。
	stop := context.AfterFunc(r.Context(), func() { _ = upstream.Close(); _ = downstream.Close() })
	defer stop()
	deadline := time.Now().Add(defaultBrowserTimeout)
	if value, ok := r.Context().Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = upstream.SetDeadline(deadline)
	_ = downstream.SetDeadline(deadline)
	if _, err := buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffer.Flush(); err != nil {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, buffer); _ = upstream.Close(); close(done) }()
	_, _ = io.Copy(downstream, upstream)
	_ = downstream.Close()
	<-done
}

// stripHopHeaders 移除逐跳请求头与代理认证头。
func stripHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, key := range strings.Split(value, ",") {
			header.Del(strings.TrimSpace(key))
		}
	}
	for _, key := range []string{"Connection", "Proxy-Connection", "Proxy-Authorization", "Proxy-Authenticate", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(key)
	}
}
