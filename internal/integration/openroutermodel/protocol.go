package openroutermodel

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"

	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// validToolName 约束模型接口接受的工具名称。
var validToolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// wireToolName 将 MCP 工具名转换为稳定的协议名称。
func wireToolName(name string) string {
	if validToolName.MatchString(name) {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("tool_%x", sum[:24])
}

// encodeTools 生成稳定工具定义与反向名称表。
func encodeTools(tools []*schema.ToolInfo) ([]components.ResponsesRequestToolUnion, map[string]string, error) {
	ordered := append([]*schema.ToolInfo(nil), tools...)
	for _, t := range ordered {
		if t == nil || t.Name == "" {
			return nil, nil, fmt.Errorf("tool name is required")
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	result := make([]components.ResponsesRequestToolUnion, 0, len(ordered))
	names := make(map[string]string, len(ordered))
	for _, t := range ordered {
		name := wireToolName(t.Name)
		if _, exists := names[name]; exists {
			return nil, nil, fmt.Errorf("duplicate tool name")
		}
		parameters, err := t.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, nil, err
		}
		var schemaValue any = parameters
		if parameters == nil {
			schemaValue = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		// 标准 JSON map 使属性序列化顺序稳定。
		encoded, err := json.Marshal(schemaValue)
		if err != nil {
			return nil, nil, err
		}
		var params map[string]any
		if err := json.Unmarshal(encoded, &params); err != nil {
			return nil, nil, err
		}
		result = append(result, components.CreateResponsesRequestToolUnionFunction(components.ResponsesRequestToolFunction{
			Name: name, Description: optionalnullable.From(&t.Desc), Parameters: params, Strict: optionalnullable.From(openrouter.Pointer(false)),
		}))
		names[name] = t.Name
	}
	return result, names, nil
}

// request 将 Eino 消息转换为完整的无状态请求。
func (m *chatModel) request(messages []*schema.Message, opts ...model.Option) (*modelRequest, map[string]string, error) {
	// Eino 通过请求选项传入本次工具定义，模型实例不保存工具状态。
	options := model.GetCommonOptions(nil, opts...)
	if options.Model != nil && *options.Model != m.config.Model {
		return nil, nil, fmt.Errorf("select models through model_tier")
	}
	if len(options.Stop) > 0 || len(options.DeferredTools) > 0 || options.ToolSearchTool != nil {
		return nil, nil, fmt.Errorf("unsupported Responses model option")
	}
	tools := options.Tools
	if len(options.AllowedToolNames) > 0 {
		allowed := map[string]bool{}
		for _, name := range options.AllowedToolNames {
			allowed[name] = true
		}
		tools = nil
		for _, tool := range options.Tools {
			if tool != nil && allowed[tool.Name] {
				tools = append(tools, tool)
				delete(allowed, tool.Name)
			}
		}
		if len(allowed) > 0 {
			return nil, nil, fmt.Errorf("unknown allowed tool")
		}
	}
	encodedTools, names, err := encodeTools(tools)
	if err != nil {
		return nil, nil, err
	}
	input := make([]any, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			return nil, nil, fmt.Errorf("nil model input")
		}
		if len(msg.MultiContent) > 0 || len(msg.UserInputMultiContent) > 0 || len(msg.AssistantGenMultiContent) > 0 {
			return nil, nil, fmt.Errorf("OpenRouter adapter currently accepts text and function tools only")
		}
		if raw, ok := msg.Extra[modelmeta.ProtocolKey].(string); ok && raw != "" && msg.Role == schema.Assistant {
			var p protocol
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				return nil, nil, fmt.Errorf("invalid model protocol history")
			}
			if p.Version == 1 && p.Endpoint == m.config.BaseURL && p.Model == m.config.Model && len(p.Output) > 0 {
				for _, out := range p.Output {
					input = append(input, out)
				}
				continue
			}
		}
		switch msg.Role {
		case schema.System, schema.User, schema.Assistant:
			if msg.Content != "" {
				input = append(input, map[string]any{"role": string(msg.Role), "content": msg.Content})
			}
			for _, call := range msg.ToolCalls {
				if call.ID == "" || call.Function.Name == "" || !json.Valid([]byte(call.Function.Arguments)) {
					return nil, nil, fmt.Errorf("invalid tool call history")
				}
				input = append(input, map[string]any{"type": "function_call", "call_id": call.ID, "name": wireToolName(call.Function.Name), "arguments": call.Function.Arguments})
			}
		case schema.Tool:
			if msg.ToolCallID == "" {
				return nil, nil, fmt.Errorf("tool result requires call ID")
			}
			input = append(input, map[string]any{"type": "function_call_output", "call_id": msg.ToolCallID, "output": msg.Content})
		default:
			return nil, nil, fmt.Errorf("unsupported model role %q", msg.Role)
		}
	}
	req := &modelRequest{Input: input, Options: components.ResponsesRequest{
		Model: &m.config.Model, Stream: openrouter.Pointer(true), Tools: encodedTools,
		Include:   optionalnullable.From(openrouter.Pointer([]components.ResponseIncludesEnum{"reasoning.encrypted_content"})),
		Reasoning: optionalnullable.From(&components.ReasoningConfig{Summary: optionalnullable.From(openrouter.Pointer(components.ReasoningSummaryVerbosity("auto")))}),
		// 显式赋默认值，避免 SDK v0.7.132 的默认字符串序列化问题。
		ServiceTier: optionalnullable.From(openrouter.Pointer(components.ResponsesRequestServiceTier("auto"))),
	}}
	if m.config.Effort != "" {
		reasoning, _ := req.Options.Reasoning.Get()
		reasoning.Effort = optionalnullable.From(openrouter.Pointer(components.ReasoningEffort(m.config.Effort)))
	}
	if options.Temperature != nil {
		req.Options.Temperature = optionalnullable.From(openrouter.Pointer(float64(*options.Temperature)))
	}
	if options.TopP != nil {
		req.Options.TopP = optionalnullable.From(openrouter.Pointer(float64(*options.TopP)))
	}
	if options.MaxTokens != nil {
		req.Options.MaxOutputTokens = optionalnullable.From(openrouter.Pointer(int64(*options.MaxTokens)))
	}
	if options.ToolChoice != nil {
		switch *options.ToolChoice {
		case schema.ToolChoiceAllowed:
			req.Options.ToolChoice = openrouter.Pointer(components.CreateOpenAIResponsesToolChoiceUnionOpenAIResponsesToolChoiceAuto("auto"))
		case schema.ToolChoiceForbidden:
			req.Options.ToolChoice = openrouter.Pointer(components.CreateOpenAIResponsesToolChoiceUnionOpenAIResponsesToolChoiceNone("none"))
		case schema.ToolChoiceForced:
			req.Options.ToolChoice = openrouter.Pointer(components.CreateOpenAIResponsesToolChoiceUnionOpenAIResponsesToolChoiceRequired("required"))
		default:
			return nil, nil, fmt.Errorf("unsupported tool choice")
		}
	}
	return req, names, nil
}

// message 将完整响应映射成可执行、可持久化的消息。
func (m *chatModel) message(r response, names map[string]string) (*schema.Message, error) {
	if r.Status != "completed" || (len(r.Error) > 0 && string(r.Error) != "null") {
		return nil, fmt.Errorf("OpenRouter response did not complete (status %q)", r.Status)
	}
	msg := schema.AssistantMessage("", nil)
	for _, raw := range r.Output {
		var out item
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		switch out.Type {
		case "message":
			for _, part := range out.Content {
				msg.Content += part.Text
				if part.Type == "refusal" {
					msg.Content += part.Refusal
				}
			}
		case "reasoning":
			// 可见推理由 readEvents 结合已发送的增量统一补齐；原始输出仍保留用于重放。
		case "function_call":
			name, ok := names[out.Name]
			if !ok || out.CallID == "" || !json.Valid([]byte(out.Arguments)) {
				return nil, fmt.Errorf("OpenRouter returned invalid or unbound function call")
			}
			idx := len(msg.ToolCalls)
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{Index: &idx, ID: out.CallID, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: out.Arguments}})
		default:
			return nil, fmt.Errorf("unsupported OpenRouter output item %q", out.Type)
		}
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
		return nil, fmt.Errorf("OpenRouter returned no text or tool calls")
	}
	encoded, err := json.Marshal(protocol{Version: 1, Endpoint: m.config.BaseURL, Model: m.config.Model, Output: r.Output})
	if err != nil {
		return nil, err
	}
	msg.Extra = map[string]any{modelmeta.ProtocolKey: string(encoded), "openrouter-response-id": r.ID}
	msg.ResponseMeta = &schema.ResponseMeta{FinishReason: "stop"}
	if len(msg.ToolCalls) > 0 {
		msg.ResponseMeta.FinishReason = "tool_calls"
	}
	if r.Usage != nil {
		u := r.Usage
		msg.ResponseMeta.Usage = &schema.TokenUsage{PromptTokens: u.Input, CompletionTokens: u.Output, TotalTokens: u.Total}
		if u.InputDetails.Cached != nil {
			msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens = *u.InputDetails.Cached
		}
		raw, _ := json.Marshal(u)
		msg.Extra["openrouter-usage"] = string(raw)
		logs.Infof("OpenRouter model=%s response=%s usage=%s", m.config.Model, r.ID, string(raw))
	}
	return msg, nil
}
