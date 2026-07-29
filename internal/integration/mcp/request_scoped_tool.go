// 请求级 Token 注入、未授权短路、传输错误处理、代理包装。
package mcp

import (
	"context"
	"errors"
	"fmt"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const tongjiAccessTokenHeader = "X-Tongji-Access-Token"

// requestScopedTool 在基础 BaseTool 基础上添加请求级鉴权能力。
type requestScopedTool struct {
	delegate tool.InvokableTool
}

// Info 返回底层 MCP Tool 的元数据。
func (t *requestScopedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.delegate.Info(ctx)
}

// InvokableRun 使用当前请求上下文中的校园访问凭据调用底层 MCP Tool。
func (t *requestScopedTool) InvokableRun(ctx context.Context, argumentsInJSON string, options ...tool.Option) (string, error) {
	accessToken, ok := platformauth.AccessTokenFromContext(ctx)
	if !ok {
		return `{"status":"unauthorized","message":"请先完成同济账号授权后再查询个人数据。"}`, nil
	}

	headers := map[string]string{tongjiAccessTokenHeader: accessToken}
	options = append(options, einoext.WithCustomHeaders(headers))
	result, err := t.delegate.InvokableRun(ctx, argumentsInJSON, options...)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	return stableToolResultJSON(toolStatusForInvocationError(err)), nil
}

// wrapRequestScopedTools 将 Eino 暴露的 BaseTool 列表逐个包装为带请求级鉴权能力的 Tool。
func wrapRequestScopedTools(tools []tool.BaseTool) ([]tool.BaseTool, error) {
	wrappedTools := make([]tool.BaseTool, 0, len(tools))
	for _, baseTool := range tools {
		invokable, ok := baseTool.(tool.InvokableTool)
		if !ok {
			return nil, fmt.Errorf("MCP tool does not support synchronous invocation")
		}
		wrappedTools = append(wrappedTools, &requestScopedTool{delegate: invokable})
	}
	return wrappedTools, nil
}
