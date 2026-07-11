package agent

import (
	"context"
	"fmt"
	"os"

	"code.byted.org/gopkg/logs"
	"code.byted.org/inf/bytedai-go/region"
	bytedclient "code.byted.org/inf/bytedmcp/go/client"
	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/compose"
)

var (
	deepAgent adk.Agent
	mcpClient *bytedclient.BytedMCPClient
)

// initChatModel reads endpoint configuration from environment variables and creates an Ark ChatModel instance.
func initChatModel(ctx context.Context) (*ark.ChatModel, error) {
	endpointID := os.Getenv("ENDPOINT_ID")
	endpointAPIKey := os.Getenv("ENDPOINT_API_KEY")
	if endpointID == "" || endpointAPIKey == "" {
		return nil, fmt.Errorf("ENDPOINT_ID or ENDPOINT_API_KEY is not set")
	}

	arkBaseUrl := region.GetBaseUrl()
	if arkBaseUrl == "" {
		return nil, fmt.Errorf("failed to get ark base url")
	}

	logs.Infof("about to initialize model with endpoint id: %s, base url: %s", endpointID, arkBaseUrl)
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		Model:   endpointID,
		APIKey:  endpointAPIKey,
		BaseURL: arkBaseUrl,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ark chat model: %w", err)
	}

	return chatModel, nil
}

func InitDeepAgentAndMcpClient(ctx context.Context) error {
	chatModel, err := initChatModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to init chat model: %w", err)
	}

	mcpClient, err = getMCPClient()
	if err != nil {
		return fmt.Errorf("failed to init mcp client: %w", err)
	}

	tools, err := getEinoTools(ctx, mcpClient)
	if err != nil {
		return fmt.Errorf("failed to get eino tools: %w", err)
	}

	filesystemMW, err := newFileSystemMiddleware(ctx)
	if err != nil {
		return fmt.Errorf("failed to create filesystem middleware: %w", err)
	}

	deepAgent, err = deep.New(ctx, &deep.Config{
		Name:        "AIPass Deep Agent",
		Description: "This is a Deep Agent powered by the AI Pass platform. It analyzes user input and dispatches tasks to the appropriate sub-agents for execution.",
		ChatModel:   chatModel,
		Handlers:    []adk.ChatModelAgentMiddleware{filesystemMW},
		ToolsConfig: adk.ToolsConfig{
			EmitInternalEvents: true,
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create deep agent: %w", err)
	}

	return nil
}

func GetDeepAgent(_ context.Context) (adk.Agent, error) {
	if deepAgent == nil {
		return nil, fmt.Errorf("deep agent not initialized")
	}
	return deepAgent, nil
}

func GetRunner(ctx context.Context, enableStreaming bool, store compose.CheckPointStore) (*adk.Runner, error) {
	a, err := GetDeepAgent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get deep agent, err: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a,
		EnableStreaming: enableStreaming,
		CheckPointStore: store,
	})
	return runner, nil
}

func GetMcpClient(_ context.Context) (*bytedclient.BytedMCPClient, error) {
	if mcpClient == nil {
		return nil, fmt.Errorf("mcp client not initialized")
	}
	return mcpClient, nil
}

// CloseMcpClient closes the global MCP client and releases its resources.
func CloseMcpClient() error {
	if mcpClient == nil {
		return nil
	}
	return mcpClient.Close()
}

// newLocalSandBox creates a local sandbox implementation that implements the filesystem.Backend interface.
// By registering this backend with the deep agent, it gains essential file operation capabilities
// including read_file, write_file, edit_file, glob, and grep.
// It also implements the filesystem.StreamingShell interface, enabling streaming shell command execution.
// If you need to use the AI Sandbox capability provided by AI PaaS, please refer to:
// https://bytedance.larkoffice.com/wiki/C85Fw9hQWiL1OAkN2w5cu8OSnV2
func newLocalSandBox(ctx context.Context) (*local.Local, error) {
	return local.NewBackend(ctx, &local.Config{})
}

func newFileSystemMiddleware(ctx context.Context) (adk.ChatModelAgentMiddleware, error) {
	localSandbox, err := newLocalSandBox(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create local sandbox, err: %v", err)
	}
	return filesystem.New(ctx, &filesystem.MiddlewareConfig{
		Backend:        localSandbox,
		StreamingShell: localSandbox,
	})
}
