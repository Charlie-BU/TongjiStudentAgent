// 请求级 Token 注入、原始调用错误传递、代理包装。
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

// requestScopedTool 为可复用的 Eino Tool 添加请求级凭据注入和原始错误传递。
// 不保存用户凭据，不检查 Skill，也不编排瑞幸登录或业务调用顺序。
type requestScopedTool struct {
	delegate tool.InvokableTool
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
			return err.Error(), nil
		}
		headers[userIDHeader] = userID
		headers[tongjiAccessTokenHeader] = accessToken
	} else if sessionID := modelmeta.SessionID(ctx); sessionID != "" {
		// 匿名身份由已校验的会话确定，不接受模型参数提供的用户 ID。
		headers[userIDHeader] = "anonymous_" + sessionID
	}
	options = append(options, einoext.WithCustomHeaders(headers))
	result, err := t.delegate.InvokableRun(ctx, argumentsInJSON, options...)
	// 保留成功调用的原始 MCP 结果。
	if err == nil {
		return result, nil
	}
	// 用户取消或整个 Run 超时应继续向上传播，不能伪装成可继续执行的业务结果。
	// 因此这两种上下文错误不作为工具结果交给模型。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	// 将原始异常文本作为工具结果交给下游模型，包括 SDK 包装的 MCP isError 内容。
	// 返回 nil error 使模型能读取异常并继续处理，而非让工具执行器直接终止整轮。
	return err.Error(), nil
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
