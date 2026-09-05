package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
)

const maxResponseBytes = 2 << 20

// Client 复用 HTTP 连接并仅向 Tavily 发送专用凭据。
type Client struct {
	key  string
	http *http.Client
}

// NewClient 创建客户端，可注入离线测试用传输层。
func NewClient(key string, timeout time.Duration, transport http.RoundTripper) (*Client, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY must be set when Tavily is enabled")
	}
	if timeout <= 0 || timeout > 2*time.Minute {
		return nil, fmt.Errorf("Tavily client timeout must be within (0, 2m]")
	}
	return &Client{key: key, http: &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// post 执行有界请求且不自动重试。
func (c *Client) post(ctx context.Context, path string, input, output any) (err error) {
	started := time.Now()
	code := 0
	defer func() {
		status := "ok"
		if err != nil {
			status = Status(err)
		}
		logging.CtxInfo(ctx, "tavily operation=%s http_status=%d status=%s duration_ms=%d", path, code, status, time.Since(started).Milliseconds())
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.http == nil {
		return &Error{Status: "web_unavailable"}
	}
	body, err := json.Marshal(input)
	if err != nil {
		return &Error{Status: "invalid_arguments"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com"+path, bytes.NewReader(body))
	if err != nil {
		return &Error{Status: "invalid_arguments"}
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
			return &Error{Status: "timeout"}
		}
		return &Error{Status: "web_unavailable"}
	}
	defer response.Body.Close()
	code = response.StatusCode
	if code != http.StatusOK {
		return &Error{Status: statusForHTTP(code), HTTPStatus: code}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
			return &Error{Status: "timeout"}
		}
		return &Error{Status: "web_unavailable"}
	}
	if len(data) > maxResponseBytes || json.Unmarshal(data, output) != nil {
		return &Error{Status: "web_unavailable"}
	}
	return nil
}
