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

	. "github.com/smartystreets/goconvey/convey"
)

func TestLuckinAllowlistCanBeDiscoveredAndInvoked(t *testing.T) {
	Convey("瑞幸完整工具清单可发现并使用 API Key 调用", t, func() {
		ctx := modelmeta.WithSession(context.Background(), "ses_fixture_a")
		srv := server.NewMCPServer("luckin-integration", "1")
		var names []string
		for _, name := range toolallowlist.MCPTools() {
			if !strings.HasPrefix(name, "luckin.") {
				continue
			}
			names = append(names, name)
			srv.AddTool(protocol.NewTool(name), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
				if req.Header.Get("Authorization") != "Bearer "+testMCPAPIKey || req.Header.Get(userIDHeader) != "" || req.Header.Get(tongjiAccessTokenHeader) != "" || req.Header.Get("X-Tongji-User-Id") != "" {
					t.Errorf("unexpected request identity for %s", req.Params.Name)
				}
				if req.Params.Name == "luckin.auth.check" {
					return protocol.NewToolResultText(`{"valid":true}`), nil
				}
				return protocol.NewToolResultText(`{"status":"ok","data":{"content":[{"type":"text","text":"{\"orderIdStr\":\"1234567890123456789\"}"}]},"source":"Luckin Coffee"}`), nil
			})
		}
		So(names, ShouldHaveLength, 11)
		httpServer := server.NewTestStreamableHTTPServer(srv)
		defer httpServer.Close()
		client := newTestRemoteClient(t, httpServer.URL)
		defer client.Close()
		tools, err := EinoTools(ctx, client, names...)
		So(err, ShouldBeNil)
		for _, base := range tools {
			info, _ := base.Info(ctx)
			value, err := base.(tool.InvokableTool).InvokableRun(ctx, `{}`)
			So(err, ShouldBeNil)
			if info.Name == "luckin.auth.check" {
				var envelope struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				}
				So(json.Unmarshal([]byte(value), &envelope), ShouldBeNil)
				So(envelope.Content, ShouldHaveLength, 1)
				So(envelope.Content[0].Text, ShouldContainSubstring, `"valid":true`)
			} else {
				So(value, ShouldContainSubstring, "1234567890123456789")
			}
		}
	})
}

func TestAnonymousLuckinUsesConfiguredAPIKeyAcrossSessions(t *testing.T) {
	Convey("不同匿名会话均使用配置的 MCP API Key", t, func() {
		srv := server.NewMCPServer("anonymous-luckin", "1")
		var ids []string
		srv.AddTool(protocol.NewTool("luckin.auth.check"), func(_ context.Context, req protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			ids = append(ids, req.Header.Get("Authorization"))
			if req.Header.Get(userIDHeader) != "" {
				t.Error("anonymous user sent an untrusted user ID")
			}
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
		So(err, ShouldBeNil)
		for _, session := range []string{"ses_a", "ses_a", "ses_b"} {
			_, err = tools[0].(tool.InvokableTool).InvokableRun(modelmeta.WithSession(context.Background(), session), `{}`)
			So(err, ShouldBeNil)
		}
		So(ids, ShouldResemble, []string{"Bearer " + testMCPAPIKey, "Bearer " + testMCPAPIKey, "Bearer " + testMCPAPIKey})
	})
}
