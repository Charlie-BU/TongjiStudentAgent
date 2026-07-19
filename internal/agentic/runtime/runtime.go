// Package runtime 封装当前单 Agent 的 Eino 运行时。
package runtime

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Config 描述运行时所需的通用 Agent 依赖。
type Config struct {
	Name        string
	Description string
	ChatModel   model.BaseChatModel
	Tools       []tool.BaseTool
	Handlers    []adk.ChatModelAgentMiddleware
}

// Runtime 持有已初始化的 Agent。
type Runtime struct {
	agent adk.Agent
}

// New 根据通用依赖创建当前单 Agent Runtime。
func New(ctx context.Context, cfg Config) (*Runtime, error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat model is required")
	}

	agent, err := deep.New(ctx, &deep.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		ChatModel:   cfg.ChatModel,
		Handlers:    cfg.Handlers,
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
	return &Runtime{agent: agent}, nil
}

// Chat 执行单轮查询并返回最终 Assistant 文本。
func (r *Runtime) Chat(ctx context.Context, message string) (string, error) {
	if r == nil || r.agent == nil {
		return "", fmt.Errorf("agent runtime is not initialized")
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: r.agent})
	iter := runner.Query(ctx, message)
	var response string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", fmt.Errorf("run agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.Role != schema.Assistant {
			continue
		}

		output, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return "", fmt.Errorf("read agent response: %w", err)
		}
		if output != nil && output.Content != "" {
			response = output.Content
		}
	}
	if response == "" {
		return "", fmt.Errorf("agent returned no text response")
	}
	return response, nil
}
