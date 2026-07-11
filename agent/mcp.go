package agent

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	einoext "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"

	"code.byted.org/gopkg/logs/v2"
	bytedclient "code.byted.org/inf/bytedmcp/go/client"
	"code.byted.org/lang/gg/gslice"
)

const (
	mcpServerPSM = "inf.sinf.sample_mcp"
)

func getMCPClientConfigs() []bytedclient.MCPClientConf {
	mcpPsm := os.Getenv("MY_MCP_PSM")
	if mcpPsm == "" {
		return []bytedclient.MCPClientConf{
			{
				ServerName: mcpServerPSM,
				ClientType: bytedclient.HTTP,
			},
		}
	}
	return gslice.Map(strings.Split(mcpPsm, ","), func(psm string) bytedclient.MCPClientConf {
		return bytedclient.MCPClientConf{
			ServerName: psm,
			ClientType: bytedclient.HTTP,
		}
	})
}

// for examples on how to use the bytedmcp go sdk, can refer to this link:
// https://code.byted.org/inf/bytedmcp/tree/master/go/examples
//
// NOTE: there is also an example on how to run the client on a local device (missing zti and vregion information)
func getMCPClient() (*bytedclient.BytedMCPClient, error) {
	// if you want to pass sensitive headers and params in tool calls, can refer to the following document:
	// https://bytedance.sg.larkoffice.com/docx/GjZbdt84yoaRPMxqILrlfthMgVd

	return bytedclient.NewBytedMCPClient(
		getMCPClientConfigs(),
		// set the request timeout to prevent any requests to the mcp server from indefinitely hanging
		bytedclient.WithRequestTimeout(15*time.Second),
		// enable logging out call tool trace, turn this off to prevent log clutter
		bytedclient.WithCallToolTraceEnabled(),
		// the mcp gateway that is accessed is inferred from the vregion that the service is deployed in
		// can add the WithMCPGatewayRegion client option if you want to force access to a specific mcp gateway
	)
}

// getEinoTools converts MCP tools from all registered MCP clients into Eino-compatible tools.
func getEinoTools(ctx context.Context, cli *bytedclient.BytedMCPClient, toolNames ...string) ([]tool.BaseTool, error) {
	if cli == nil {
		return nil, errors.New("cannot convert MCP tools to Eino tools: client cannot be nil")
	}
	var einoTools []tool.BaseTool
	for _, client := range cli.ListMCPClients() {
		conf := &einoext.Config{
			Cli:          client,
			ToolNameList: toolNames,
			// Extension point: customize how tool call results are processed before returning to the model.
			// By default this passes through the original result unchanged.
			ToolCallResultHandler: func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{
					Content: result.Content,
					IsError: result.IsError,
				}, nil
			},
		}
		tools, err := einoext.GetTools(ctx, conf)
		if err != nil {
			logs.CtxError(ctx, "failed to get eino tools from MCP client: %v", err)
			return nil, err
		}
		einoTools = append(einoTools, tools...)
	}
	return einoTools, nil
}
