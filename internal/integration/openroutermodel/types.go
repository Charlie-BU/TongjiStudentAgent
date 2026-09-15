package openroutermodel

import (
	"encoding/json"
	"net/http"

	"github.com/OpenRouterTeam/go-sdk/models/components"
)

// ── 模型实例与连接配置：供模型构造使用 ──

// chatModel 将 OpenRouter Responses 调用适配为 Eino 模型。
type chatModel struct {
	config config
}

// config 描述 OpenRouter 的共享连接配置。
type config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Effort     string
	HTTPClient *http.Client
}

// ── 请求、响应与历史协议 ──

// protocol 保存可重放的完整输出及来源边界。
type protocol struct {
	Version  int               `json:"version"`
	Endpoint string            `json:"endpoint"`
	Model    string            `json:"model"`
	Output   []json.RawMessage `json:"output"`
}

// response 描述 Responses 的完成对象。
type response struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
	Error  json.RawMessage   `json:"error"`
	Usage  *usage            `json:"usage"`
}

// usage 保留输入输出及可选缓存统计。
type usage struct {
	Input        int `json:"input_tokens"`
	Output       int `json:"output_tokens"`
	Total        int `json:"total_tokens"`
	InputDetails struct {
		Cached  *int `json:"cached_tokens"`
		Written *int `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
}

// item 提取输出中 Eino 需要的字段。
type item struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	} `json:"content"`
	Summary []reasoningPart `json:"summary"`
}

// modelRequest 将类型化选项与必须无损重放的原始历史分开。
type modelRequest struct {
	Options components.ResponsesRequest
	Input   []any
}

// ── 推理摘要与流进度 ──

// reasoningPart 兼容字符串与 summary_text 对象两种摘要结构。
type reasoningPart struct {
	Text string `json:"text"`
}

// reasoningPartJSON 用于摘要对象解码，避免递归调用 UnmarshalJSON。
type reasoningPartJSON reasoningPart

// reasoningProgress 记录单个输出项已经展示的推理表示。
type reasoningProgress struct{ Kind, Text string }

// ── SSE 事件 ──

// streamEvent 描述 Responses SSE 数据事件。
type streamEvent struct {
	OutputIndex int      `json:"output_index"`
	Type        string   `json:"type"`
	Delta       string   `json:"delta"`
	Response    response `json:"response"`
}

// streamEnvelope 对应 SDK EventStream 提供的事件外层结构。
type streamEnvelope struct {
	Data streamEvent `json:"data"`
}

// ── SDK 传输：仅保存单次请求的协议和响应句柄 ──

// protocolClient 注入原始 input 并交出流，不维护跨请求状态。
type protocolClient struct {
	client   *http.Client
	input    json.RawMessage
	response *http.Response
}
