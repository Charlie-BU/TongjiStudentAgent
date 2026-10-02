// 环境配置、远程 Client 创建、启动与初始化。
package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	remoteClientName    = "TongjiStudentMCPServer"
	remoteClientVersion = "0.1.0"
)

// RemoteConfig 描述远程 MCP Client 的连接配置。
type RemoteConfig struct {
	ServerURL string
}

// RemoteConfigFromEnv 从环境变量读取远程 MCP 连接配置。
func RemoteConfigFromEnv() (RemoteConfig, error) {
	serverURL := strings.TrimSpace(os.Getenv("MCP_SERVER_URL"))
	if serverURL == "" {
		return RemoteConfig{}, fmt.Errorf("MCP_SERVER_URL is required")
	}
	parsedURL, err := url.ParseRequestURI(serverURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return RemoteConfig{}, fmt.Errorf("MCP_SERVER_URL must be an absolute http or https URL")
	}

	return RemoteConfig{ServerURL: serverURL}, nil
}

// NewRemoteClientFromEnv 从环境变量创建并初始化远程 MCP Client。
func NewRemoteClientFromEnv(ctx context.Context) (*mcpclient.Client, error) {
	config, err := RemoteConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewRemoteClient(ctx, config)
}

// NewRemoteClient 创建、启动并初始化远程 Streamable HTTP MCP Client。
func NewRemoteClient(ctx context.Context, config RemoteConfig) (*mcpclient.Client, error) {
	client, err := mcpclient.NewStreamableHttpClient(config.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("create remote MCP client: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start remote MCP client: %w", err)
	}
	if _, err := client.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    remoteClientName,
				Version: remoteClientVersion,
			},
		},
	}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize remote MCP client: %w", err)
	}
	return client, nil
}
