package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestGetCurrentTimeTool(t *testing.T) {
	ctx := context.Background()
	client, err := NewLocalClient(ctx)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	result, err := client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "get_current_time"},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("unexpected tool result: %#v", result)
	}
}
