package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/eino/components/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	githubmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	. "github.com/smartystreets/goconvey/convey"
)

const testMCPToolName = "tongji.student.score"

func TestRemoteConfigFromEnv(t *testing.T) {
	Convey("远程 MCP 连接配置", t, func() {
		Convey("合法环境变量应生成连接配置", func() {
			t.Setenv("MCP_SERVER_URL", "https://mcp.example.test/mcp")
			t.Setenv("MCP_TIMEOUT", "12s")

			config, err := RemoteConfigFromEnv()

			So(err, ShouldBeNil)
			So(config.ServerURL, ShouldEqual, "https://mcp.example.test/mcp")
			So(config.Timeout, ShouldEqual, 12*time.Second)
		})

		Convey("缺失或非法环境变量应被拒绝", func() {
			for _, test := range []struct {
				serverURL string
				timeout   string
			}{
				{serverURL: "", timeout: "12s"},
				{serverURL: "localhost:3000/mcp", timeout: "12s"},
				{serverURL: "ftp://mcp.example.test/mcp", timeout: "12s"},
				{serverURL: "https://mcp.example.test/mcp", timeout: ""},
				{serverURL: "https://mcp.example.test/mcp", timeout: "invalid"},
				{serverURL: "https://mcp.example.test/mcp", timeout: "0s"},
			} {
				t.Setenv("MCP_SERVER_URL", test.serverURL)
				t.Setenv("MCP_TIMEOUT", test.timeout)

				_, err := RemoteConfigFromEnv()

				So(err, ShouldNotBeNil)
			}
		})
	})
}

func TestNewRemoteClientInitializationFailure(t *testing.T) {
	Convey("远程 MCP 初始化失败", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		Convey("应关闭失败连接并返回初始化错误", func() {
			client, err := NewRemoteClient(context.Background(), RemoteConfig{ServerURL: server.URL, Timeout: time.Second})

			So(client, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "initialize remote MCP client")
		})
	})
}

func TestRequestScopedMCPTool(t *testing.T) {
	Convey("请求级 MCP Tool 包装器", t, func() {
		var receivedTokens []string
		var receivedTokensMu sync.Mutex
		mcpServer := server.NewMCPServer("test-mcp-server", "1.0.0")
		mcpServer.AddTool(githubmcp.NewTool(testMCPToolName), func(_ context.Context, request githubmcp.CallToolRequest) (*githubmcp.CallToolResult, error) {
			receivedTokensMu.Lock()
			receivedTokens = append(receivedTokens, request.Header.Get(tongjiAccessTokenHeader))
			receivedTokensMu.Unlock()
			return githubmcp.NewToolResultText("score result"), nil
		})
		testServer := server.NewTestStreamableHTTPServer(mcpServer)
		defer testServer.Close()

		client := newTestRemoteClient(t, testServer.URL)
		defer client.Close()
		tools, err := EinoTools(context.Background(), client, testMCPToolName)
		So(err, ShouldBeNil)
		So(tools, ShouldHaveLength, 1)
		invokable, ok := tools[0].(tool.InvokableTool)
		So(ok, ShouldBeTrue)

		Convey("缺失凭据时不应发起 MCP 请求", func() {
			result, invokeErr := invokable.InvokableRun(context.Background(), `{}`)

			So(invokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"unauthorized"`)
			receivedTokensMu.Lock()
			So(receivedTokens, ShouldBeEmpty)
			receivedTokensMu.Unlock()
		})

		Convey("allowlist 工具缺失时应拒绝启动", func() {
			tools, toolsErr := EinoTools(context.Background(), client, "not-registered")

			So(tools, ShouldBeNil)
			So(toolsErr, ShouldNotBeNil)
			So(toolsErr.Error(), ShouldContainSubstring, "allowlist")
		})

		Convey("应仅为本次调用注入凭据", func() {
			requestContext := platformauth.WithAccessToken(context.Background(), "test-access-token")
			result, invokeErr := invokable.InvokableRun(requestContext, `{}`)
			secondRequestContext := platformauth.WithAccessToken(context.Background(), "another-access-token")
			secondResult, secondInvokeErr := invokable.InvokableRun(secondRequestContext, `{}`)

			So(invokeErr, ShouldBeNil)
			So(secondInvokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, "score result")
			So(secondResult, ShouldContainSubstring, "score result")
			So(result, ShouldNotContainSubstring, "test-access-token")
			So(secondResult, ShouldNotContainSubstring, "another-access-token")
			receivedTokensMu.Lock()
			So(receivedTokens, ShouldResemble, []string{"test-access-token", "another-access-token"})
			receivedTokensMu.Unlock()
		})
	})
}

// newTestRemoteClient 创建连接到离线 MCP 测试服务的 Client。
func newTestRemoteClient(t *testing.T, serverURL string) *mcpclient.Client {
	t.Helper()
	client, err := NewRemoteClient(context.Background(), RemoteConfig{ServerURL: serverURL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("create remote MCP client: %v", err)
	}
	return client
}
