// Package runtime 封装当前单 Agent 的 Eino 运行时。
package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Config 描述运行时所需的通用 Agent 依赖。
type Config struct {
	Name          string
	Description   string
	Instruction   string
	SkillCatalog  string
	ChatModel     model.BaseChatModel
	Tools         []tool.BaseTool
	MaxIterations int
	Handlers      []adk.ChatModelAgentMiddleware
}

// Runtime 持有已初始化的 DeepAgent。
type Runtime struct {
	agent        adk.Agent
	skillCatalog string
}

// New 根据通用依赖创建当前单 Agent Runtime。
func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat model is required")
	}

	agent, err := deep.New(ctx, &deep.Config{
		Name:                   cfg.Name,
		Description:            cfg.Description,
		Instruction:            cfg.Instruction,
		ChatModel:              cfg.ChatModel,
		MaxIteration:           cfg.MaxIterations,
		Handlers:               cfg.Handlers,
		WithoutWriteTodos:      true, // 使用 system.manage_task_plan 管理任务计划
		WithoutGeneralSubAgent: true,
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

// TODO：待拆解 tool call 处理
// StreamWithHistory 执行单轮查询，并将 canonical 会话历史作为模型上下文注入输入。
func (r *Runtime) StreamWithHistory(ctx context.Context, query, studentInfo string, history []agenticsession.Message, emit func(agentevent.Event)) (string, error) {
	return r.StreamWithHistoryAndMemory(ctx, query, studentInfo, history, "", emit)
}

// StreamWithHistoryAndMemory 执行查询，并将已压缩的历史摘要注入动态提醒。
func (r *Runtime) StreamWithHistoryAndMemory(ctx context.Context, query, studentInfo string, history []agenticsession.Message, summary string, emit func(agentevent.Event)) (string, error) {
	return r.StreamWithHistoryAndMessagesAndMemory(ctx, query, studentInfo, history, summary, emit, nil)
}

// StreamWithHistoryAndMessages 执行查询，并将运行时输出逐条交给调用方持久化。
func (r *Runtime) StreamWithHistoryAndMessages(ctx context.Context, query, studentInfo string, history []agenticsession.Message, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error) {
	return r.StreamWithHistoryAndMessagesAndMemory(ctx, query, studentInfo, history, "", emit, record)
}

// StreamWithHistoryAndMessagesAndMemory 执行查询，并持久化运行过程中的完整消息。
func (r *Runtime) StreamWithHistoryAndMessagesAndMemory(ctx context.Context, query, studentInfo string, history []agenticsession.Message, summary string, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error) {
	if r == nil || r.agent == nil {
		return "", fmt.Errorf("agent runtime is not initialized")
	}
	if emit == nil {
		emit = func(agentevent.Event) {}
	}

	messages, err := buildInputMessagesWithHistory(ctx, query, studentInfo, r.skillCatalog, summary, time.Now(), history)
	if err != nil {
		return "", fmt.Errorf("build agent input: %w", err)
	}
	runCtx := agenticskills.WithRunState(ctx, agenticskills.NewRunState())
	runner := adk.NewRunner(runCtx, adk.RunnerConfig{Agent: r.agent, EnableStreaming: true})
	iter := runner.Run(runCtx, messages)
	var response string
	pendingTools := make(map[string]agentevent.ToolCallStartedData)
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
		if record != nil {
			if err := record(ctx, output); err != nil {
				return "", fmt.Errorf("record agent message: %w", err)
			}
		}
		switch event.Output.MessageOutput.Role {
		case schema.Assistant:
			if output.ReasoningContent != "" {
				emit(agentevent.Event{Type: agentevent.AssistantReasoning, Data: agentevent.AssistantReasoningData{Text: output.ReasoningContent}})
			}
			for _, toolCall := range output.ToolCalls {
				if toolCall.ID == "" || toolCall.Function.Name == "" {
					continue
				}
				if _, exists := pendingTools[toolCall.ID]; exists {
					continue
				}
				toolData := agentevent.ToolCallStartedData{CallID: toolCall.ID, Tool: toolCall.Function.Name, DisplayName: toolCall.Function.Name, Arguments: toolCall.Function.Arguments}
				pendingTools[toolCall.ID] = toolData
				toolStartedAt[toolCall.ID] = time.Now()
				emit(agentevent.Event{Type: agentevent.ToolCallStarted, Data: toolData})
			}
			if output.Content != "" {
				response = output.Content
			}
		case schema.Tool:
			toolData, exists := pendingTools[output.ToolCallID]
			if !exists {
				toolData = agentevent.ToolCallStartedData{CallID: output.ToolCallID, Tool: output.ToolName, DisplayName: output.ToolName}
			}
			emit(agentevent.Event{Type: agentevent.ToolCallCompleted, Data: agentevent.ToolCallCompletedData{
				CallID: toolData.CallID, Tool: toolData.Tool, DurationMS: elapsedMilliseconds(toolStartedAt[output.ToolCallID]), Result: output.Content,
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

// failPendingTools 处理未完成的工具调用，将它们标记为失败。
func failPendingTools(emit func(agentevent.Event), pendingTools map[string]agentevent.ToolCallStartedData, toolStartedAt map[string]time.Time) {
	for callID, toolCall := range pendingTools {
		emit(agentevent.Event{Type: agentevent.ToolCallFailed, Data: agentevent.ToolCallFailedData{
			CallID: callID, Tool: toolCall.Tool, DurationMS: elapsedMilliseconds(toolStartedAt[callID]),
			Code: "agent_execution_failed", Message: "工具调用未完成",
		}})
	}
}

// readMessage 读取 Deep Agent 的输出消息，根据是否为流式响应进行处理。
// 它会将流式响应转换为非流式响应，同时触发 `emit` 事件。
func readMessage(output *adk.MessageVariant, emit func(agentevent.Event)) (*schema.Message, error) {
	if !output.IsStreaming {
		if output.Message != nil && output.Role == schema.Assistant && output.Message.Content != "" {
			emit(agentevent.Event{Type: agentevent.AssistantDelta, Data: agentevent.AssistantDeltaData{Text: output.Message.Content}})
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
				emit(agentevent.Event{Type: agentevent.AssistantDelta, Data: agentevent.AssistantDeltaData{Text: message.Content}})
			}
		}
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(messages)
}

// elapsedMilliseconds 计算从指定时间开始到当前时间的毫秒数。
func elapsedMilliseconds(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	return time.Since(start).Milliseconds()
}
