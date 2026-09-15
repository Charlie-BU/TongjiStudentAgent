package openroutermodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/operations"
	"github.com/OpenRouterTeam/go-sdk/retry"
)

// send 由 SDK 负责 URL、鉴权、请求选项和 HTTP 调用。
// input/output 必须无损重放；生成式 SDK 的已知类型会丢失未知字段，
// 因此通过每次调用独立的 client 保留原始协议，流分帧使用 SDK EventStream。
func (m *chatModel) send(ctx context.Context, payload *modelRequest) (*http.Response, error) {
	input, err := json.Marshal(payload.Input)
	if err != nil {
		return nil, err
	}
	request := payload.Options
	if id := modelmeta.SessionID(ctx); id != "" {
		request.SessionID = &id
	}
	transport := &protocolClient{client: m.config.HTTPClient, input: input}
	sdk := openrouter.New(openrouter.WithSecurity(m.config.APIKey), openrouter.WithServerURL(m.config.BaseURL), openrouter.WithClient(transport), openrouter.WithRetryConfig(retry.Config{Strategy: "none"}))
	result, sdkErr := sdk.Responses.Send(ctx, request, nil, operations.WithAcceptHeaderOverride(operations.AcceptHeaderEnumTextEventStream))
	if transport.response == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("OpenRouter request failed")
	}
	resp := transport.response
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		// SDK 错误可能包含上游正文；保持现有脱敏边界。
		return nil, fmt.Errorf("OpenRouter HTTP %d", resp.StatusCode)
	}
	if sdkErr != nil || result == nil || result.EventStream == nil {
		resp.Body.Close()
		return nil, fmt.Errorf("OpenRouter response decoding failed")
	}
	return resp, nil
}

func (c *protocolClient) Do(req *http.Request) (*http.Response, error) {
	// 用原始 input 替换 SDK 的类型化 input，避免重放时丢失扩展字段。
	var body map[string]json.RawMessage
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		req.Body.Close()
		return nil, err
	}
	req.Body.Close()
	body["input"] = c.input
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	req.ContentLength = int64(len(raw))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	c.response = resp
	return resp, nil
}
