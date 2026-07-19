package mcp

import (
	"context"
	"errors"

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
	conf := &einoext.Config{
		Cli:          cli,
		ToolNameList: toolNames,
		ToolCallResultHandler: func(_ context.Context, _ string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
			return result, nil
		},
	}
	tools, err := einoext.GetTools(ctx, conf)
	if err != nil {
		return nil, err
	}
	return tools, nil
}
