package agent

import (
	"context"
	"errors"

	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Charlie-BU/TongjiStudent/mcpserver"
)

// getMCPClient creates a client for the project-local MCP Server. The previous
// Byted MCP PSM client integration is intentionally replaced by this local demo.
func getMCPClient(ctx context.Context) (*mcpclient.Client, error) {
	return mcpserver.NewClient(ctx)
}

// getEinoTools converts tools registered by the local MCP Server into Eino-compatible tools.
func getEinoTools(ctx context.Context, cli *mcpclient.Client, toolNames ...string) ([]tool.BaseTool, error) {
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
