// 环境配置、远程 Client 创建、启动与初始化。
package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	remoteClientName    = "TongjiStudentMCPServer"
	remoteClientVersion = "0.1.0"
)

// RemoteConfig 描述远程 MCP Client 的连接配置。
type RemoteConfig struct {
	ServerURL string
	APIKey    string
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

	apiKey, err := mcpAPIKeyFromEnv()
	if err != nil {
		return RemoteConfig{}, err
	}
	return RemoteConfig{ServerURL: serverURL, APIKey: apiKey}, nil
}

// mcpAPIKeyFromEnv 读取初始化、工具发现和未登录工具调用使用的 MCP 凭据。
func mcpAPIKeyFromEnv() (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("TONGJI_STUDENT_MCP_API_KEY"))
	if err := validateMCPAPIKey(apiKey); err != nil {
		return "", err
	}
	return apiKey, nil
}

func validateMCPAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("TONGJI_STUDENT_MCP_API_KEY is required")
	}
	for _, character := range apiKey {
		if character < 0x21 || character > 0x7e {
			return fmt.Errorf("TONGJI_STUDENT_MCP_API_KEY must contain only non-space ASCII characters")
		}
	}
	return nil
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
	if err := validateMCPAPIKey(config.APIKey); err != nil {
		return nil, err
	}
	client, err := mcpclient.NewStreamableHttpClient(config.ServerURL, transport.WithHTTPHeaderFunc(func(ctx context.Context) map[string]string {
		if _, loggedIn := platformauth.UserIDFromContext(ctx); loggedIn {
			return nil
		}
		return map[string]string{"Authorization": "Bearer " + config.APIKey}
	}))
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
