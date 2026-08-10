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
	"github.com/cloudwego/eino/schema"
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
			switch request.GetArguments()["scenario"] {
			case "unauthorized":
				return &githubmcp.CallToolResult{
					Content: []githubmcp.Content{githubmcp.TextContent{Type: "text", Text: `{"status":"unauthorized","message":"raw upstream authorization detail"}`}},
					IsError: true,
				}, nil
			case "unknown_error":
				return &githubmcp.CallToolResult{
					Content: []githubmcp.Content{githubmcp.TextContent{Type: "text", Text: `{"status":"unexpected","message":"raw upstream failure detail"}`}},
					IsError: true,
				}, nil
			}
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

		Convey("缺失凭据时应继续发起 MCP 请求并透传空凭据", func() {
			result, invokeErr := invokable.InvokableRun(context.Background(), `{}`)

			So(invokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, "score result")
			receivedTokensMu.Lock()
			So(receivedTokens, ShouldResemble, []string{""})
			receivedTokensMu.Unlock()
		})

		Convey("allowlist 工具缺失时应拒绝启动", func() {
			tools, toolsErr := EinoTools(context.Background(), client, "not-registered")

			So(tools, ShouldBeNil)
			So(toolsErr, ShouldNotBeNil)
			So(toolsErr.Error(), ShouldContainSubstring, "allowlist")
		})

		Convey("空白或重复 allowlist 不应发现全部远程工具", func() {
			for _, names := range [][]string{nil, {""}, {testMCPToolName, testMCPToolName}} {
				tools, toolsErr := EinoTools(context.Background(), client, names...)

				So(tools, ShouldBeNil)
				So(toolsErr, ShouldNotBeNil)
				So(toolsErr.Error(), ShouldContainSubstring, "allowlist")
			}
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

		Convey("应将 MCP 业务错误收敛为稳定结果", func() {
			requestContext := platformauth.WithAccessToken(context.Background(), "test-access-token")
			result, invokeErr := invokable.InvokableRun(requestContext, `{"scenario":"unauthorized"}`)
			unknownResult, unknownInvokeErr := invokable.InvokableRun(requestContext, `{"scenario":"unknown_error"}`)

			So(invokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, toolStatusUnauthorized)
			So(result, ShouldNotContainSubstring, "raw upstream authorization detail")
			So(unknownInvokeErr, ShouldBeNil)
			So(unknownResult, ShouldContainSubstring, toolStatusExecutionUnavailable)
			So(unknownResult, ShouldNotContainSubstring, "raw upstream failure detail")
		})
	})
}

func TestRequestScopedToolNormalizesTransportError(t *testing.T) {
	Convey("请求级 MCP Tool 传输错误", t, func() {
		wrappedTool := &requestScopedTool{delegate: testInvokableTool{run: func(context.Context, string, ...tool.Option) (string, error) {
			return "", testTimeoutError{}
		}}}

		Convey("应返回稳定超时结果且不暴露原始错误", func() {
			requestContext := platformauth.WithAccessToken(context.Background(), "test-access-token")
			result, err := wrappedTool.InvokableRun(requestContext, `{}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, toolStatusUpstreamTimeout)
			So(result, ShouldNotContainSubstring, "test timeout detail")
		})
	})
}

type testInvokableTool struct {
	run func(context.Context, string, ...tool.Option) (string, error)
}

// Info 返回测试工具的最小元数据。
func (t testInvokableTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: testMCPToolName}, nil
}

// InvokableRun 执行测试预设的工具调用。
func (t testInvokableTool) InvokableRun(ctx context.Context, argumentsInJSON string, options ...tool.Option) (string, error) {
	return t.run(ctx, argumentsInJSON, options...)
}

type testTimeoutError struct{}

func (testTimeoutError) Error() string {
	return "test timeout detail"
}

func (testTimeoutError) Timeout() bool {
	return true
}

func (testTimeoutError) Temporary() bool {
	return true
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
