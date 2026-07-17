// Package mcpserver contains MCP tools owned and registered by this service.
package mcpserver

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

// NewServer registers the MCP tools that are available to the DeepAgent.
func NewServer() *server.MCPServer {
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

// NewClient creates and initializes an in-process client for the local MCP Server.
func NewClient(ctx context.Context) (*client.Client, error) {
	mcpClient, err := client.NewInProcessClient(NewServer())
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
