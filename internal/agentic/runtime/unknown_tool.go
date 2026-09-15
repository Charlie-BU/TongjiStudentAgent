package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const unknownToolResultKey = "runtime-unknown-tool"

// unknownToolResult 将未注册工具作为可恢复结果回填，不执行调用或回显其参数。
// 模型可依据当前工具目录纠正调用，仍受 Runtime 的最大迭代次数限制。
func unknownToolResult(ctx context.Context, name, _ string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	const result = `{"status":"tool_not_found","message":"该工具未注册，本次调用未执行。请仅使用当前提供的工具定义中的准确名称，不要将返回数据字段当作工具名；如无可用工具，请说明能力限制。"}`
	// Eino 的未知工具分支绕过工具事件中间件，需显式发布结果以持久化完整调用链。
	message := schema.ToolMessage(result, compose.GetToolCallID(ctx), schema.WithToolName(name))
	message.Extra = map[string]any{unknownToolResultKey: true}
	if err := adk.SendEvent(ctx, adk.EventFromMessage(message, nil, schema.Tool, name)); err != nil {
		return "", err
	}
	return result, nil
}
