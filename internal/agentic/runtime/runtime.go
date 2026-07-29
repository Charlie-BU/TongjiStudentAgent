// Package runtime 封装当前单 Agent 的 Eino 运行时。
package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Config 描述运行时所需的通用 Agent 依赖。
type Config struct {
	Name                   string
	Description            string
	Instruction            string
	SkillCatalog           string
	ChatModel              model.BaseChatModel
	Tools                  []tool.BaseTool
	Handlers               []adk.ChatModelAgentMiddleware
	WithoutWriteTodos      bool
	WithoutGeneralSubAgent bool
}

// Runtime 持有已初始化的 Agent。
type Runtime struct {
	agent        adk.Agent
	skillCatalog string
}

type toolCallStartedData struct {
	CallID      string `json:"call_id"`
	Tool        string `json:"tool"`
	DisplayName string `json:"display_name"`
}

type toolCallCompletedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
}

type toolCallFailedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// New 根据通用依赖创建当前单 Agent Runtime。
// TODO：不合理，使用自建 Agent Graph
func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat model is required")
	}

	agent, err := deep.New(ctx, &deep.Config{
		Name:                   cfg.Name,
		Description:            cfg.Description,
		Instruction:            cfg.Instruction,
		ChatModel:              cfg.ChatModel,
		Handlers:               cfg.Handlers,
		WithoutWriteTodos:      cfg.WithoutWriteTodos,
		WithoutGeneralSubAgent: cfg.WithoutGeneralSubAgent,
		ToolsConfig: adk.ToolsConfig{
			EmitInternalEvents: true,
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: cfg.Tools,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create deep agent: %w", err)
	}
	return &Runtime{agent: agent, skillCatalog: cfg.SkillCatalog}, nil
}

// Stream 执行单轮查询，并通过 emit 输出已脱敏的模型文本与工具生命周期事件。
// 它不会输出模型 reasoning content、工具参数或工具原始响应。
func (r *Runtime) Stream(ctx context.Context, query string, emit func(agentevent.Event)) (string, error) {
	if r == nil || r.agent == nil {
		return "", fmt.Errorf("agent runtime is not initialized")
	}
	if emit == nil {
		emit = func(agentevent.Event) {}
	}

	messages, err := buildInputMessages(query, r.skillCatalog, time.Now())
	if err != nil {
		return "", fmt.Errorf("build agent input: %w", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: r.agent, EnableStreaming: true})
	iter := runner.Run(ctx, messages)
	var response string
	pendingTools := make(map[string]toolCallStartedData)
	toolStartedAt := make(map[string]time.Time)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			failPendingTools(emit, pendingTools, toolStartedAt)
			return "", fmt.Errorf("run agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		output, err := readMessage(event.Output.MessageOutput, emit)
		if err != nil {
			failPendingTools(emit, pendingTools, toolStartedAt)
			return "", fmt.Errorf("read agent response: %w", err)
		}
		if output == nil {
			continue
		}
		switch event.Output.MessageOutput.Role {
		case schema.Assistant:
			for _, toolCall := range output.ToolCalls {
				if toolCall.ID == "" || toolCall.Function.Name == "" {
					continue
				}
				if _, exists := pendingTools[toolCall.ID]; exists {
					continue
				}
				toolData := toolCallStartedData{CallID: toolCall.ID, Tool: toolCall.Function.Name, DisplayName: toolCall.Function.Name}
				pendingTools[toolCall.ID] = toolData
				toolStartedAt[toolCall.ID] = time.Now()
				emit(agentevent.Event{Type: agentevent.ToolCallStarted, Data: toolData})
			}
			if output.Content != "" {
				// 每个 Assistant 输出代表一次模型回合；保留最后一次文本，
				// 与原 JSON 接口的最终回答语义保持一致。
				response = output.Content
			}
		case schema.Tool:
			toolData, exists := pendingTools[output.ToolCallID]
			if !exists {
				toolData = toolCallStartedData{CallID: output.ToolCallID, Tool: output.ToolName, DisplayName: output.ToolName}
			}
			emit(agentevent.Event{Type: agentevent.ToolCallCompleted, Data: toolCallCompletedData{
				CallID: toolData.CallID, Tool: toolData.Tool, DurationMS: elapsedMilliseconds(toolStartedAt[output.ToolCallID]),
			}})
			delete(pendingTools, output.ToolCallID)
			delete(toolStartedAt, output.ToolCallID)
		}
	}
	if response == "" {
		failPendingTools(emit, pendingTools, toolStartedAt)
		return "", fmt.Errorf("agent returned no text response")
	}
	return response, nil
}

func failPendingTools(emit func(agentevent.Event), pendingTools map[string]toolCallStartedData, toolStartedAt map[string]time.Time) {
	for callID, toolCall := range pendingTools {
		emit(agentevent.Event{Type: agentevent.ToolCallFailed, Data: toolCallFailedData{
			CallID: callID, Tool: toolCall.Tool, DurationMS: elapsedMilliseconds(toolStartedAt[callID]),
			Code: "agent_execution_failed", Message: "工具调用未完成",
		}})
	}
}

func readMessage(output *adk.MessageVariant, emit func(agentevent.Event)) (*schema.Message, error) {
	if !output.IsStreaming {
		if output.Message != nil && output.Role == schema.Assistant && output.Message.Content != "" {
			emit(agentevent.Event{Type: agentevent.AssistantDelta, Data: map[string]string{"text": output.Message.Content}})
		}
		return output.Message, nil
	}

	var messages []*schema.Message
	for {
		message, err := output.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if message != nil {
			messages = append(messages, message)
			if output.Role == schema.Assistant && message.Content != "" {
				emit(agentevent.Event{Type: agentevent.AssistantDelta, Data: map[string]string{"text": message.Content}})
			}
		}
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(messages)
}

func elapsedMilliseconds(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	return time.Since(start).Milliseconds()
}
