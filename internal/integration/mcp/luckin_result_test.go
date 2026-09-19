package mcp

import (
	"context"
	"encoding/json"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
	protocol "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"strings"
	"testing"
)

func TestLuckinErrorsPreserveSafeRecovery(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"luckin.auth.check", `{"status":"upstream_unavailable"}`, "暂时无法检查"},
		{"luckin.auth.login", `{"status":"unauthorized","message":"瑞幸登录未成功，请核对手机号和验证码后重试。"}`, "核对手机号和验证码"},
		{"luckin.order.create", `{"status":"upstream_unavailable","message":"瑞幸订单操作结果未确认，请先核实订单状态，不要直接重复创建或取消订单。"}`, "不要直接重复"},
		{"luckin.order.create", `{"status":"unknown","message":"PRIVATE_TOKEN"}`, "不要直接重复"},
		{"luckin.shop.search", `{"status":"unauthorized","message":"PRIVATE_TOKEN"}`, "瑞幸授权"},
	} {
		result := protocol.NewToolResultText(test.input)
		result.IsError = true
		got := normalizeNamedMCPToolResult(test.name, result)
		value, _ := toolResultText(got.Content[0])
		if !strings.Contains(value, test.want) || strings.Contains(value, "PRIVATE_TOKEN") || strings.Contains(value, "校园") {
			t.Fatalf("%s: %s", test.name, value)
		}
	}
	for _, name := range []string{"luckin.order.create", "luckin.order.cancel"} {
		if !strings.Contains(namedToolFailureJSON(name, toolStatusUpstreamTimeout), "不要直接重复") {
			t.Fatal(name)
		}
	}
	if !strings.Contains(namedToolFailureJSON("luckin.auth.check", toolStatusUpstreamTimeout), "upstream_timeout") {
		t.Fatal("check must fail closed")
	}
}

func TestLuckinAllowlistCanBeDiscoveredAndInvoked(t *testing.T) {
	ctx := context.Background()
	srv := server.NewMCPServer("luckin-integration", "1")
	var names []string
	for _, name := range toolallowlist.MCPTools() {
		if !strings.HasPrefix(name, "luckin.") {
			continue
		}
		names = append(names, name)
		srv.AddTool(protocol.NewTool(name), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
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

func TestCheckErrorsDoNotBecomeLoginRequired(t *testing.T) {
	for code := range luckinCheckMessages {
		response := protocol.NewToolResultText(`{"status":"` + code + `","message":"PRIVATE_TOKEN"}`)
		response.IsError = true
		result := normalizeNamedMCPToolResult("luckin.auth.check", response)
		value, _ := toolResultText(result.Content[0])
		if !strings.Contains(value, code) || strings.Contains(value, "PRIVATE_TOKEN") || !strings.Contains(value, `"valid":false`) {
			t.Fatal(value)
		}
	}
	for _, response := range []*protocol.CallToolResult{nil, protocol.NewToolResultText(`{"valid":"true"}`), protocol.NewToolResultText(`{}`)} {
		value, _ := toolResultText(normalizeNamedMCPToolResult("luckin.auth.check", response).Content[0])
		if !strings.Contains(value, "upstream_unavailable") {
			t.Fatal(value)
		}
	}
}
