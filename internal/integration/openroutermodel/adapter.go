package openroutermodel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.BaseChatModel = (*chatModel)(nil)

// newModel 创建 OpenRouter Responses 模型。
func newModel(cfg config) (*chatModel, error) {
	cfg.Model, cfg.APIKey = strings.TrimSpace(cfg.Model), strings.TrimSpace(cfg.APIKey)
	if cfg.Model == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("model ID and OPENROUTER_API_KEY are required")
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(u.Path, "/responses") {
		return nil, fmt.Errorf("OPENROUTER_BASE_URL must be an HTTP(S) API base URL without credentials, query or /responses")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &chatModel{config: cfg}, nil
}

// StatelessResponses 是本项目的上下文装配标记：每次请求重放本轮所需的完整历史，
// 不依赖 previous_response_id 续接服务端响应链。会话历史仍由本地存储管理，
// 上游也仍可复用相同输入前缀的计算缓存；“无状态”不表示无历史或禁用缓存。
func (m *chatModel) StatelessResponses() bool { return true }

// Generate 汇总与 Stream 相同的模型调用，避免维护两套响应处理。
func (m *chatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	stream, err := m.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var messages []*schema.Message
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return schema.ConcatMessages(messages)
}
