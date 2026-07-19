// Package mcp 提供当前 MCP Client 的适配实现。
package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "tongjistudent-mcp-demo"
	serverVersion = "1.0.0"
)

// newLocalServer 注册本地开发使用的 MCP demo 工具。
func newLocalServer() *server.MCPServer {
	mcpServer := server.NewMCPServer(serverName, serverVersion)
	mcpServer.AddTool(
		mcp.NewTool(
			"get_current_time",
			mcp.WithDescription("Returns the current UTC time in RFC3339 format."),
		),
		getCurrentTime,
	)
	return mcpServer
}

// NewLocalClient 创建并初始化进程内 MCP demo Client。
func NewLocalClient(ctx context.Context) (*client.Client, error) {
	mcpClient, err := client.NewInProcessClient(newLocalServer())
	if err != nil {
		return nil, fmt.Errorf("create local MCP client: %w", err)
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("start local MCP client: %w", err)
	}

	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "tongjistudent-agent",
				Version: serverVersion,
			},
		},
	})
	if err != nil {
		_ = mcpClient.Close()
		return nil, fmt.Errorf("initialize local MCP client: %w", err)
	}
	return mcpClient, nil
}

func getCurrentTime(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	currentTime := time.Now().UTC().Format(time.RFC3339)
	return mcp.NewToolResultText(currentTime), nil
}
