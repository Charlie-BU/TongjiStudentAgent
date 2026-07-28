// Eino Tool 发现、Allowlist 校验与完整性检查。
package mcp

import (
	"context"
	"errors"
	"fmt"

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
		ToolCallResultHandler: func(_ context.Context, _ string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			return normalizeMCPToolResult(result), nil
		},
	})
	if err != nil {
		return nil, err
	}
	if len(tools) != len(toolNames) {
		return nil, fmt.Errorf("MCP tool allowlist is not fully available")
	}
	return wrapRequestScopedTools(tools)
}
