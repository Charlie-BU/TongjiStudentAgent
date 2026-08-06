package mcp

import (
	"testing"

	githubmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestNormalizeMCPToolResultDropsStructuredContent(t *testing.T) {
	result := &githubmcp.CallToolResult{
		Content:           []githubmcp.Content{githubmcp.TextContent{Type: "text", Text: "fallback result"}},
		StructuredContent: map[string]any{"student_id": "123"},
	}

	normalized := normalizeMCPToolResult(result)

	if normalized != result {
		t.Fatal("expected successful tool result to be preserved")
	}
	if normalized.StructuredContent != nil {
		t.Fatalf("structured content = %#v, want nil", normalized.StructuredContent)
	}
}
