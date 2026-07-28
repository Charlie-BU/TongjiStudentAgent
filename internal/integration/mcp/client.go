package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	remoteClientName        = "TongjiStudentMCPServer"
	remoteClientVersion     = "0.1.0"
	tongjiAccessTokenHeader = "X-Tongji-Access-Token"
)

// RemoteConfig 描述远程 MCP Client 的连接配置。
type RemoteConfig struct {
	ServerURL string
	Timeout   time.Duration
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

	timeoutValue := strings.TrimSpace(os.Getenv("MCP_TIMEOUT"))
	if timeoutValue == "" {
		return RemoteConfig{}, fmt.Errorf("MCP_TIMEOUT is required")
	}
	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil || timeout <= 0 {
		return RemoteConfig{}, fmt.Errorf("MCP_TIMEOUT must be a positive duration")
	}

	return RemoteConfig{ServerURL: serverURL, Timeout: timeout}, nil
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
	client, err := mcpclient.NewStreamableHttpClient(
		config.ServerURL,
		transport.WithHTTPTimeout(config.Timeout),
	)
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
	if len(toolNames) > 0 && len(tools) != len(toolNames) {
		return nil, fmt.Errorf("MCP tool allowlist is not fully available")
	}
	return wrapRequestScopedTools(tools)
}

// requestScopedTool 在基础 BaseTool 基础上添加请求级鉴权能力。
type requestScopedTool struct {
	delegate tool.InvokableTool
}

// Info 返回底层 MCP Tool 的元数据。
func (t *requestScopedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.delegate.Info(ctx)
}

// InvokableRun 使用当前请求上下文中的校园访问凭据调用底层 MCP Tool。
func (t *requestScopedTool) InvokableRun(ctx context.Context, argumentsInJSON string, options ...tool.Option) (string, error) {
	accessToken, ok := platformauth.AccessTokenFromContext(ctx)
	if !ok {
		return `{"status":"unauthorized","message":"请先完成同济账号授权后再查询个人数据。"}`, nil
	}

	headers := map[string]string{tongjiAccessTokenHeader: accessToken}
	options = append(options, einoext.WithCustomHeaders(headers))
	return t.delegate.InvokableRun(ctx, argumentsInJSON, options...)
}

// wrapRequestScopedTools 将 Eino 暴露的 BaseTool 列表逐个包装为带请求级鉴权能力的 Tool。
// 返回值保持为 []tool.BaseTool，是为了与上游 einoext.GetTools 的接口契约一致，
// 同时对调用方隐藏具体包装实现；实际追加到切片中的元素是 *requestScopedTool。
// 若某个工具不支持同步调用（未实现 tool.InvokableTool），则返回错误。
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
