// 请求级 Token 注入、传输错误处理、代理包装。
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

// InvokableRun 使用当前请求上下文中的校园访问凭据调用底层 MCP Tool。
func (t *requestScopedTool) InvokableRun(ctx context.Context, argumentsInJSON string, options ...tool.Option) (string, error) {
	// 每次从当前请求读取同济凭据，不能缓存到 Tool 实例，否则并发用户可能串号。
	// 缺少凭据也允许请求到达 MCP Server，由具体工具决定是否需要身份。
	accessToken, _ := platformauth.AccessTokenFromContext(ctx)

	// 注入的是同济 access_token，不是瑞幸 Bearer。瑞幸 Token 由 MCP Server 按用户查询。
	// 使用本次调用的 options，避免修改共享 MCP Client 的默认 Header。
	headers := map[string]string{tongjiAccessTokenHeader: accessToken}
	options = append(options, einoext.WithCustomHeaders(headers))
	result, err := t.delegate.InvokableRun(ctx, argumentsInJSON, options...)
	// err=nil 只表示调用层正常返回；业务失败也可能已被结果处理器转成稳定 JSON。
	if err == nil {
		return result, nil
	}
	// 用户取消或整个 Run 超时应继续向上传播，不能伪装成可继续执行的业务结果。
	// 因此这两种上下文错误不走下方 check=false 等降级分支。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	// 其他调用失败不暴露原始错误：check 返回 false，订单写操作提示先核实、不要重试；
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
