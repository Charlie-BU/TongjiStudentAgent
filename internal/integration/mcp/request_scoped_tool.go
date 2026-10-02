// 请求级 Token 注入、传输错误处理、代理包装。
package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const tongjiAccessTokenHeader = "X-Tongji-Access-Token"
const userIDHeader = "X-User-Id"

// requestScopedTool 为可复用的 Eino Tool 添加请求级凭据注入和传输失败归一。
// 不保存用户凭据，不检查 Skill，也不编排瑞幸登录或业务调用顺序。
type requestScopedTool struct {
	delegate tool.InvokableTool // 实际执行 MCP 调用；收到的业务结果由 ToolCallResultHandler 处理。
	name     string             // 发现工具时保存的名称，用于在没有 MCP 响应时选择对应的失败语义。
}

// Info 返回底层 MCP Tool 的元数据。
func (t *requestScopedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.delegate.Info(ctx)
}

// InvokableRun 为同济用户注入服务凭据，为匿名会话注入用户标识；身份校验由 MCP 执行。
func (t *requestScopedTool) InvokableRun(ctx context.Context, argumentsInJSON string, options ...tool.Option) (string, error) {
	headers := map[string]string{}
	if userID, loggedIn := platformauth.UserIDFromContext(ctx); loggedIn {
		accessToken, err := tongjiapi.MCPAccessToken(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return namedToolFailureJSON(t.name, toolStatusExecutionUnavailable), nil
		}
		headers[userIDHeader] = userID
		headers[tongjiAccessTokenHeader] = accessToken
	} else if sessionID := modelmeta.SessionID(ctx); sessionID != "" {
		// 匿名身份由已校验的会话确定，不接受模型参数提供的用户 ID。
		headers[userIDHeader] = "anonymous_" + sessionID
	}
	options = append(options, einoext.WithCustomHeaders(headers))
	result, err := t.delegate.InvokableRun(ctx, argumentsInJSON, options...)
	// err=nil 只表示调用层正常返回；业务失败也可能已被结果处理器转成稳定 JSON。
	if err == nil {
		return result, nil
	}
	// 用户取消或整个 Run 超时应继续向上传播，不能伪装成可继续执行的业务结果。
	// 因此这两种上下文错误不走下方 业务错误归一分支。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	// 其他调用失败不暴露原始错误：check 返回检查失败提示，订单写操作提示先核实、不要重试；
	// 非瑞幸工具沿用校园服务提示。这里只返回恢复信息，不实际重试或额外调用 check。
	return namedToolFailureJSON(t.name, toolStatusForInvocationError(err)), nil
}

// wrapRequestScopedTools 将 Eino 暴露的 BaseTool 列表逐个包装为带请求级鉴权能力的 Tool。
func wrapRequestScopedTools(tools []tool.BaseTool) ([]tool.BaseTool, error) {
	wrappedTools := make([]tool.BaseTool, 0, len(tools))
	for _, baseTool := range tools {
		invokable, ok := baseTool.(tool.InvokableTool)
		if !ok {
			return nil, fmt.Errorf("MCP tool does not support synchronous invocation")
		}
		// 启动装配时读取已发现的工具元数据；此处没有用户请求上下文，也不获取用户凭据。
		info, err := baseTool.Info(context.Background())
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, fmt.Errorf("MCP tool info is nil")
		}
		wrappedTools = append(wrappedTools, &requestScopedTool{delegate: invokable, name: info.Name})
	}
	return wrappedTools, nil
}
