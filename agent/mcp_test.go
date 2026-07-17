package agent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
)

func TestGetEinoToolsFromLocalMCPServer(t *testing.T) {
	ctx := context.Background()
	client, err := getMCPClient(ctx)
	if err != nil {
		t.Fatalf("getMCPClient() error = %v", err)
	}
	defer client.Close()

	tools, err := getEinoTools(ctx, client)
	if err != nil {
		t.Fatalf("getEinoTools() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tool count = %d, want 1", len(tools))
	}

	invokable, ok := tools[0].(tool.InvokableTool)
	if !ok {
		t.Fatal("local MCP tool is not invokable")
	}
	result, err := invokable.InvokableRun(ctx, "{}")
	if err != nil || result == "" {
		t.Fatalf("InvokableRun() result = %q, error = %v", result, err)
	}
}
