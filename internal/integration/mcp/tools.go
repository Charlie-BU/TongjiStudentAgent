// Eino Tool 发现、Allowlist 校验与完整性检查。
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// EinoTools 将 MCP Client 暴露的工具转换为 Eino 工具。
func EinoTools(ctx context.Context, cli *mcpclient.Client, toolNames ...string) ([]tool.BaseTool, error) {
	if cli == nil {
		return nil, errors.New("cannot convert MCP tools to Eino tools: client cannot be nil")
	}
	if err := toolallowlist.ValidateToolAllowlist(toolNames); err != nil {
		return nil, err
	}
	tools, err := einoext.GetTools(ctx, &einoext.Config{
		Cli:          cli,
		ToolNameList: toolNames,
		// 已收到 MCP 结果（包括 isError）时按工具名归一；未取得结果的调用错误
		// 由 requestScopedTool 处理，避免将瑞幸授权/订单错误改写为校园服务错误。
		ToolCallResultHandler: func(_ context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			return normalizeNamedMCPToolResult(name, result), nil
		},
	})
	if err != nil {
		return nil, err
	}
	available := make(map[string]struct{}, len(tools))
	for _, discoveredTool := range tools {
		info, err := discoveredTool.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("read discovered MCP tool info: %w", err)
		}
		if info == nil {
			return nil, errors.New("discovered MCP tool info is nil")
		}
		available[info.Name] = struct{}{}
	}
	var missing []string
	for _, name := range toolNames {
		if _, ok := available[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 || len(tools) != len(toolNames) {
		return nil, fmt.Errorf("MCP tool allowlist is not fully available: expected=%d, discovered=%d, missing=[%s]; check MCP_SERVER_URL and deploy matching MCP server and Agent versions", len(toolNames), len(tools), strings.Join(missing, ", "))
	}
	return wrapRequestScopedTools(tools)
}
