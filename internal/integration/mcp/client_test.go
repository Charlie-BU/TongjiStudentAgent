package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
	githubmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	. "github.com/smartystreets/goconvey/convey"
)

const testMCPToolName = "tongji.bachelor.score"
const testMCPAPIKey = "test-mcp-api-key-for-anonymous-users"

func TestRemoteConfigFromEnv(t *testing.T) {
	t.Setenv("TONGJI_STUDENT_MCP_API_KEY", testMCPAPIKey)
	Convey("远程 MCP 连接配置", t, func() {
		Convey("合法环境变量应生成连接配置", func() {
			t.Setenv("MCP_SERVER_URL", "https://mcp.example.test/mcp")

			config, err := RemoteConfigFromEnv()

			So(err, ShouldBeNil)
			So(config.ServerURL, ShouldEqual, "https://mcp.example.test/mcp")
			So(config.APIKey, ShouldEqual, testMCPAPIKey)
		})

		Convey("缺失或非法环境变量应被拒绝", func() {
			for _, serverURL := range []string{
				"",
				"localhost:3000/mcp",
				"ftp://mcp.example.test/mcp",
			} {
				t.Setenv("MCP_SERVER_URL", serverURL)

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
			client, err := NewRemoteClient(context.Background(), RemoteConfig{ServerURL: server.URL, APIKey: testMCPAPIKey})

			So(client, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "initialize remote MCP client")
		})
	})
}

func TestMCPAPIKeyConfiguration(t *testing.T) {
	Convey("MCP API Key 配置", t, func() {
		t.Setenv("MCP_SERVER_URL", "https://mcp.example.test/mcp")
		for _, apiKey := range []string{"", "  ", "key with space", "key\r\nheader", "密钥"} {
			Convey(fmt.Sprintf("拒绝非法 Key %q，且不发起未登录调用", apiKey), func() {
				t.Setenv("TONGJI_STUDENT_MCP_API_KEY", apiKey)
				_, err := RemoteConfigFromEnv()
				So(err, ShouldNotBeNil)
				invoked := false
				wrapped := &requestScopedTool{delegate: testInvokableTool{run: func(context.Context, string, ...tool.Option) (string, error) {
					invoked = true
					return "", nil
				}}}
				result, err := wrapped.InvokableRun(context.Background(), `{}`)
				So(err, ShouldBeNil)
				So(invoked, ShouldBeFalse)
				So(result, ShouldContainSubstring, "TONGJI_STUDENT_MCP_API_KEY")
			})
		}
		Convey("合法 Key 应去除首尾空白", func() {
			t.Setenv("TONGJI_STUDENT_MCP_API_KEY", "  "+testMCPAPIKey+"  ")
			config, err := RemoteConfigFromEnv()
			So(err, ShouldBeNil)
			So(config.APIKey, ShouldEqual, testMCPAPIKey)
		})
	})
}

func TestNewRemoteClientAuthenticatesInitializationAndDiscovery(t *testing.T) {
	Convey("MCP 初始化和工具发现均携带 API Key", t, func() {
		srv := server.NewMCPServer("authenticated-mcp", "1")
		srv.AddTool(githubmcp.NewTool("luckin.auth.check"), func(context.Context, githubmcp.CallToolRequest) (*githubmcp.CallToolResult, error) {
			return githubmcp.NewToolResultText(`{"valid":false}`), nil
		})
		handler := server.NewStreamableHTTPServer(srv)
		var mu sync.Mutex
		requests := 0
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+testMCPAPIKey {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			mu.Lock()
			requests++
			mu.Unlock()
			handler.ServeHTTP(w, r)
		}))
		defer httpServer.Close()
		client := newTestRemoteClient(t, httpServer.URL)
		defer client.Close()
		_, err := EinoTools(context.Background(), client, "luckin.auth.check")
		So(err, ShouldBeNil)
		mu.Lock()
		defer mu.Unlock()
		So(requests, ShouldBeGreaterThanOrEqualTo, 2)
	})
}

func TestRequestScopedMCPTool(t *testing.T) {
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			fmt.Fprint(w, `{"access_token":"service-token","expires_in":7200}`)
			return
		}
		if r.Header.Get("Authorization") == "Bearer service-token" {
			fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"00001","name":"李建中","userTypeName":"教职工"}]}}`)
			return
		}
		id := "student-a"
		if r.Header.Get("Authorization") == "Bearer another-access-token" {
			id = "student-b"
		}
		fmt.Fprintf(w, `{"code":"A00000","data":{"list":[{"userId":%q}]}}`, id)
	}))
	defer identityServer.Close()
	for key, value := range map[string]string{"TONGJI_LOGIN_CLIENT_ID": "login", "TONGJI_LOGIN_CLIENT_SECRET": "secret", "TONGJI_OPEN_PLATFORM_REDIRECT_URI": "https://example.test", "TONGJI_OPEN_PLATFORM_STATE_SECRET": "state", "TONGJI_OPEN_PLATFORM_API_BASE_URL": identityServer.URL} {
		t.Setenv(key, value)
	}
	t.Setenv("TONGJI_MCP_CLIENT_ID", "service")
	t.Setenv("TONGJI_MCP_CLIENT_SECRET", "secret")
	t.Setenv("TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT", identityServer.URL+"/token")

	Convey("请求级 MCP Tool 包装器", t, func() {
		var receivedTokens []string
		var receivedUsers []string
		var receivedBearers []string
		var receivedTokensMu sync.Mutex
		mcpServer := server.NewMCPServer("test-mcp-server", "1.0.0")
		mcpServer.AddTool(githubmcp.NewTool(testMCPToolName), func(_ context.Context, request githubmcp.CallToolRequest) (*githubmcp.CallToolResult, error) {
			receivedTokensMu.Lock()
			receivedTokens = append(receivedTokens, request.Header.Get(tongjiAccessTokenHeader))
			receivedUsers = append(receivedUsers, request.Header.Get(userIDHeader))
			receivedBearers = append(receivedBearers, request.Header.Get("Authorization"))
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

		mcpServer.AddTool(githubmcp.NewTool("luckin.auth.check"), func(_ context.Context, request githubmcp.CallToolRequest) (*githubmcp.CallToolResult, error) {
			receivedTokensMu.Lock()
			receivedTokens = append(receivedTokens, request.Header.Get(tongjiAccessTokenHeader))
			receivedUsers = append(receivedUsers, request.Header.Get(userIDHeader))
			receivedBearers = append(receivedBearers, request.Header.Get("Authorization"))
			receivedTokensMu.Unlock()
			return githubmcp.NewToolResultText(`{"valid":true}`), nil
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

		Convey("未登录时应携带环境变量中的 API Key，且不发送同济身份头", func() {
			result, invokeErr := invokable.InvokableRun(context.Background(), `{}`)

			So(invokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, "score result")
			receivedTokensMu.Lock()
			So(receivedTokens, ShouldResemble, []string{""})
			So(receivedUsers, ShouldResemble, []string{""})
			So(receivedBearers, ShouldResemble, []string{"Bearer " + testMCPAPIKey})
			receivedTokensMu.Unlock()
		})

		Convey("allowlist 工具缺失时应拒绝启动", func() {
			tools, toolsErr := EinoTools(context.Background(), client, "not-registered")

			So(tools, ShouldBeNil)
			So(toolsErr, ShouldNotBeNil)
			So(toolsErr.Error(), ShouldContainSubstring, "allowlist")
			So(toolsErr.Error(), ShouldContainSubstring, "missing=[not-registered]")
		})

		Convey("部分工具缺失时应准确列出缺项并拒绝启动", func() {
			tools, toolsErr := EinoTools(context.Background(), client, testMCPToolName, "tongji.course.reviews", "tongji.course.summary")

			So(tools, ShouldBeNil)
			So(toolsErr, ShouldNotBeNil)
			So(toolsErr.Error(), ShouldContainSubstring, "expected=3, discovered=1")
			So(toolsErr.Error(), ShouldContainSubstring, "missing=[tongji.course.reviews, tongji.course.summary]")
			So(toolsErr.Error(), ShouldContainSubstring, "MCP_SERVER_URL")
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
			So(receivedTokens, ShouldResemble, []string{"service-token", "service-token"})
			So(receivedUsers, ShouldResemble, []string{"student-a", "student-b"})
			So(receivedBearers, ShouldResemble, []string{"", ""})
			receivedTokensMu.Unlock()
		})

		Convey("同一客户端交替处理未登录和已登录调用时凭据不得串用", func() {
			loggedInContext := platformauth.WithAccessToken(context.Background(), "test-access-token")
			for _, ctx := range []context.Context{context.Background(), loggedInContext, context.Background()} {
				_, err := invokable.InvokableRun(ctx, `{}`)
				So(err, ShouldBeNil)
			}
			receivedTokensMu.Lock()
			defer receivedTokensMu.Unlock()
			So(receivedTokens, ShouldResemble, []string{"", "service-token", ""})
			So(receivedUsers, ShouldResemble, []string{"", "student-a", ""})
			So(receivedBearers, ShouldResemble, []string{"Bearer " + testMCPAPIKey, "", "Bearer " + testMCPAPIKey})
		})

		Convey("服务凭据获取失败不得下传用户 token 或发起工具请求", func() {
			t.Setenv("TONGJI_MCP_CLIENT_SECRET", "")
			defer t.Setenv("TONGJI_MCP_CLIENT_SECRET", "secret")
			result, err := invokable.InvokableRun(platformauth.WithAccessToken(context.Background(), "test-access-token"), `{}`)
			So(err, ShouldBeNil)
			So(result, ShouldEqual, "TONGJI_MCP_CLIENT_ID and TONGJI_MCP_CLIENT_SECRET are required")
			So(receivedTokens, ShouldBeEmpty)
		})

		Convey("同济用户的瑞幸调用携带服务 Token，获取失败时不发起工具请求", func() {
			luckinTools, err := EinoTools(context.Background(), client, "luckin.auth.check")
			So(err, ShouldBeNil)
			ctx := platformauth.WithAccessToken(context.Background(), "test-access-token")
			_, err = luckinTools[0].(tool.InvokableTool).InvokableRun(ctx, `{}`)
			So(err, ShouldBeNil)
			t.Setenv("TONGJI_MCP_CLIENT_SECRET", "")
			defer t.Setenv("TONGJI_MCP_CLIENT_SECRET", "secret")
			value, err := luckinTools[0].(tool.InvokableTool).InvokableRun(ctx, `{}`)
			So(err, ShouldBeNil)
			So(value, ShouldEqual, "TONGJI_MCP_CLIENT_ID and TONGJI_MCP_CLIENT_SECRET are required")
			So(receivedUsers, ShouldResemble, []string{"student-a"})
			So(receivedTokens, ShouldResemble, []string{"service-token"})
		})

		Convey("应保留 MCP 业务错误的原始状态和诊断内容", func() {
			requestContext := platformauth.WithAccessToken(context.Background(), "test-access-token")
			result, invokeErr := invokable.InvokableRun(requestContext, `{"scenario":"unauthorized"}`)
			unknownResult, unknownInvokeErr := invokable.InvokableRun(requestContext, `{"scenario":"unknown_error"}`)

			So(invokeErr, ShouldBeNil)
			So(result, ShouldContainSubstring, "unauthorized")
			So(result, ShouldContainSubstring, "raw upstream authorization detail")
			So(unknownInvokeErr, ShouldBeNil)
			So(unknownResult, ShouldContainSubstring, "unexpected")
			So(unknownResult, ShouldContainSubstring, "raw upstream failure detail")
		})
	})
}

func TestRequestScopedToolPreservesInvocationErrors(t *testing.T) {
	Convey("MCP 调用错误保留原始文本", t, func() {
		t.Setenv("TONGJI_STUDENT_MCP_API_KEY", testMCPAPIKey)
		for _, invocationError := range []error{
			testTimeoutError{},
			errors.New("request failed with status 403: invalid_service_credential"),
			fmt.Errorf("MCP request: %w", errors.New("connection refused")),
		} {
			Convey(invocationError.Error(), func() {
				wrappedTool := &requestScopedTool{delegate: testInvokableTool{run: func(context.Context, string, ...tool.Option) (string, error) {
					return "", invocationError
				}}}
				result, err := wrappedTool.InvokableRun(context.Background(), `{}`)
				So(err, ShouldBeNil)
				So(result, ShouldEqual, invocationError.Error())
			})
		}
	})
}

func TestRequestScopedToolPropagatesCancellation(t *testing.T) {
	Convey("MCP 调用取消和超时继续向上传播", t, func() {
		t.Setenv("TONGJI_STUDENT_MCP_API_KEY", testMCPAPIKey)
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
			Convey(cause.Error(), func() {
				invocationError := fmt.Errorf("MCP request: %w", cause)
				wrappedTool := &requestScopedTool{delegate: testInvokableTool{run: func(context.Context, string, ...tool.Option) (string, error) {
					return "", invocationError
				}}}
				result, err := wrappedTool.InvokableRun(context.Background(), `{}`)
				So(result, ShouldBeEmpty)
				So(errors.Is(err, cause), ShouldBeTrue)
			})
		}
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
	t.Setenv("TONGJI_STUDENT_MCP_API_KEY", testMCPAPIKey)
	client, err := NewRemoteClient(context.Background(), RemoteConfig{ServerURL: serverURL, APIKey: testMCPAPIKey})
	if err != nil {
		t.Fatalf("create remote MCP client: %v", err)
	}
	return client
}
