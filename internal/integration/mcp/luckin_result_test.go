package mcp

import (
	"context"
	"encoding/json"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
	protocol "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"strings"
	"testing"
)

func TestLuckinAllowlistCanBeDiscoveredAndInvoked(t *testing.T) {
	ctx := modelmeta.WithSession(context.Background(), "ses_fixture_a")
	srv := server.NewMCPServer("luckin-integration", "1")
	var names []string
	for _, name := range toolallowlist.MCPTools() {
		if !strings.HasPrefix(name, "luckin.") {
			continue
		}
		names = append(names, name)
		srv.AddTool(protocol.NewTool(name), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			if req.Header.Get(userIDHeader) != "anonymous_ses_fixture_a" || req.Header.Get(tongjiAccessTokenHeader) != "" || req.Header.Get("X-Tongji-User-Id") != "" {
				t.Errorf("unexpected request identity for %s", req.Params.Name)
			}
			if req.Params.Name == "luckin.auth.check" {
				return protocol.NewToolResultText(`{"valid":true}`), nil
			}
			return protocol.NewToolResultText(`{"status":"ok","data":{"content":[{"type":"text","text":"{\"orderIdStr\":\"1234567890123456789\"}"}]},"source":"Luckin Coffee"}`), nil
		})
	}
	if len(names) != 11 {
		t.Fatalf("registered %d", len(names))
	}
	httpServer := server.NewTestStreamableHTTPServer(srv)
	defer httpServer.Close()
	client := newTestRemoteClient(t, httpServer.URL)
	defer client.Close()
	tools, err := EinoTools(ctx, client, names...)
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range tools {
		info, _ := base.Info(ctx)
		value, err := base.(tool.InvokableTool).InvokableRun(ctx, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		if info.Name == "luckin.auth.check" {
			var envelope struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			}
			if json.Unmarshal([]byte(value), &envelope) != nil || len(envelope.Content) != 1 || !strings.Contains(envelope.Content[0].Text, `"valid":true`) {
				t.Fatal(value)
			}
		} else if !strings.Contains(value, "1234567890123456789") {
			t.Fatal(value)
		}
	}
}

func TestAnonymousLuckinIdentityIsStableAndIsolated(t *testing.T) {
	srv := server.NewMCPServer("anonymous-luckin", "1")
	var ids []string
	srv.AddTool(protocol.NewTool("luckin.auth.check"), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		ids = append(ids, req.Header.Get(userIDHeader))
		if req.Header.Get(tongjiAccessTokenHeader) != "" {
			t.Error("anonymous user sent Tongji token")
		}
		return protocol.NewToolResultText(`{"valid":false}`), nil
	})
	httpServer := server.NewTestStreamableHTTPServer(srv)
	defer httpServer.Close()
	client := newTestRemoteClient(t, httpServer.URL)
	defer client.Close()
	tools, err := EinoTools(context.Background(), client, "luckin.auth.check")
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"ses_a", "ses_a", "ses_b"} {
		_, err = tools[0].(tool.InvokableTool).InvokableRun(modelmeta.WithSession(context.Background(), session), `{}`)
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(ids, ",") != "anonymous_ses_a,anonymous_ses_a,anonymous_ses_b" {
		t.Fatalf("unexpected identities: %v", ids)
	}
}
