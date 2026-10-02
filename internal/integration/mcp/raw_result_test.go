package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	"github.com/cloudwego/eino/components/tool"
	protocol "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMCPBusinessResultsRetainOriginalContent(t *testing.T) {
	srv := server.NewMCPServer("raw-results", "1")
	names := []string{"luckin.auth.check", "luckin.auth.send_sms_code", testMCPToolName}
	for _, name := range names {
		srv.AddTool(protocol.NewTool(name), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			if req.GetArguments()["scenario"] == "error" {
				return &protocol.CallToolResult{
					Content: []protocol.Content{protocol.TextContent{
						Type: "text",
						Text: `{"valid":false,"status":"provider_specific_error","message":"raw provider diagnostic","detail":{"request_id":"request-42"}}`,
					}},
					StructuredContent: map[string]any{"diagnostic": "original structured error"},
					IsError:           true,
				}, nil
			}
			return &protocol.CallToolResult{
				Content:           []protocol.Content{protocol.TextContent{Type: "text", Text: "original success text"}},
				StructuredContent: map[string]any{"diagnostic": "original structured success"},
			}, nil
		})
	}
	httpServer := server.NewTestStreamableHTTPServer(srv)
	defer httpServer.Close()
	client := newTestRemoteClient(t, httpServer.URL)
	defer client.Close()
	ctx := modelmeta.WithSession(context.Background(), "raw_result_fixture")
	tools, err := EinoTools(ctx, client, names...)
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range tools {
		info, _ := base.Info(ctx)
		t.Run(info.Name, func(t *testing.T) {
			invokable := base.(tool.InvokableTool)
			result, err := invokable.InvokableRun(ctx, `{"scenario":"error"}`)
			if err != nil {
				t.Fatal(err)
			}
			for _, original := range []string{"provider_specific_error", "raw provider diagnostic", "request-42", "original structured error", `"isError":true`} {
				if !strings.Contains(result, original) {
					t.Fatalf("original error detail %q missing from model tool result: %s", original, result)
				}
			}
			result, err = invokable.InvokableRun(ctx, `{}`)
			if err != nil || !strings.Contains(result, "original success text") || !strings.Contains(result, "original structured success") {
				t.Fatalf("original success content was not preserved: (%s, %v)", result, err)
			}
		})
	}
}

func TestMCPHTTPFailureReachesAgentWithStatusAndBody(t *testing.T) {
	srv := server.NewMCPServer("raw-http-error", "1")
	invoked := false
	srv.AddTool(protocol.NewTool("luckin.auth.send_sms_code"), func(context.Context, protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		invoked = true
		return protocol.NewToolResultText("should not be invoked"), nil
	})
	handler := server.NewStreamableHTTPServer(srv)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(body, &request) == nil && request.Method == "tools/call" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"invalid_service_credential","message":"original credential validation detail"}`)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	client := newTestRemoteClient(t, httpServer.URL)
	defer client.Close()
	ctx := modelmeta.WithSession(context.Background(), "raw_http_fixture")
	tools, err := EinoTools(ctx, client, "luckin.auth.send_sms_code")
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools[0].(tool.InvokableTool).InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("HTTP error should reach the model as tool content: %v", err)
	}
	for _, original := range []string{"status 403", "invalid_service_credential", "original credential validation detail"} {
		if !strings.Contains(result, original) {
			t.Fatalf("original HTTP detail %q missing from model tool result: %s", original, result)
		}
	}
	if invoked {
		t.Fatal("HTTP rejection should not execute the SMS tool")
	}
}
