package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	"github.com/cloudwego/hertz/pkg/app"
	. "github.com/smartystreets/goconvey/convey"
)

const testTongjiStateSecret = "test-state-secret"

type tokenRequestCapture struct {
	method      string
	contentType string
	form        url.Values
}

func TestTongjiAuthorize(t *testing.T) {
	Convey("同济 OAuth 授权入口", t, func() {
		Convey("配置完整时应重定向到开放平台并附带有效 state", func() {
			setTongjiOAuthEnv(t, "https://open-platform.test/authorize", "https://open-platform.test/token")
			requestContext := app.NewContext(0)

			TongjiAuthorize(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusFound)
			redirectURL, err := url.Parse(string(requestContext.Response.Header.Peek("Location")))
			So(err, ShouldBeNil)
			So(redirectURL.Host, ShouldEqual, "open-platform.test")
			So(redirectURL.Query().Get("client_id"), ShouldEqual, "test-client-id")
			So(redirectURL.Query().Get("redirect_uri"), ShouldEqual, "https://app.tongji.edu.cn/wallbreakerAuth/callback.html")
			stateClient, err := tongjiapi.New(testTongjiConfig("https://open-platform.test/authorize", "https://open-platform.test/token"))
			So(err, ShouldBeNil)
			So(stateClient.ValidateState(redirectURL.Query().Get("state")), ShouldBeNil)
		})

		Convey("缺少配置时应返回不泄露凭据的服务错误", func() {
			clearTongjiOAuthEnv(t)
			requestContext := app.NewContext(0)

			TongjiAuthorize(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusInternalServerError)
			So(responseError(requestContext), ShouldEqual, "Tongji Open Platform is not configured")
		})
	})
}

func TestTongjiExchangeToken(t *testing.T) {
	Convey("同济 OAuth 令牌交换", t, func() {
		Convey("请求体非法、字段为空或 state 无效时应拒绝请求", func() {
			setTongjiOAuthEnv(t, "https://open-platform.test/authorize", "https://open-platform.test/token")
			tests := []struct {
				name      string
				body      string
				wantError string
			}{
				{name: "非法 JSON", body: `{`, wantError: "request body must be valid JSON"},
				{name: "缺少 code", body: `{"state":"state"}`, wantError: "code and state are required"},
				{name: "伪造 state", body: `{"code":"code","state":"forged.state"}`, wantError: "invalid or expired OAuth state"},
			}

			for _, test := range tests {
				test := test
				Convey(test.name, func() {
					requestContext := newTongjiJSONRequest(test.body, tongjiCallbackOrigin)

					TongjiExchangeToken(context.Background(), requestContext)

					So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
					So(responseError(requestContext), ShouldEqual, test.wantError)
					So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Origin")), ShouldEqual, tongjiCallbackOrigin)
				})
			}
		})

		Convey("上游令牌交换失败时应返回网关错误", func() {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			setTongjiOAuthEnv(t, "https://open-platform.test/authorize", server.URL)
			state := newTongjiState(t, server.URL)
			requestContext := newTongjiJSONRequest(`{"code":"valid-code","state":"`+state+`"}`, tongjiCallbackOrigin)

			TongjiExchangeToken(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadGateway)
			So(responseError(requestContext), ShouldEqual, "exchange authorization code failed")
		})

		Convey("有效请求应转发正确表单并且不返回 refresh token", func() {
			requests := make(chan tokenRequestCapture, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_ = request.ParseForm()
				requests <- tokenRequestCapture{
					method:      request.Method,
					contentType: request.Header.Get("Content-Type"),
					form:        request.Form,
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","expires_in":300,"scope":"openid user","refresh_token":"must-not-leak"}`))
			}))
			defer server.Close()
			setTongjiOAuthEnv(t, "https://open-platform.test/authorize", server.URL)
			state := newTongjiState(t, server.URL)
			requestContext := newTongjiJSONRequest(`{"code":" valid-code ","state":" `+state+` "}`, tongjiCallbackOrigin)

			TongjiExchangeToken(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Origin")), ShouldEqual, tongjiCallbackOrigin)
			request := <-requests
			So(request.method, ShouldEqual, http.MethodPost)
			So(request.contentType, ShouldEqual, "application/x-www-form-urlencoded")
			So(request.form.Get("grant_type"), ShouldEqual, "authorization_code")
			So(request.form.Get("code"), ShouldEqual, "valid-code")
			So(request.form.Get("client_id"), ShouldEqual, "test-client-id")
			So(request.form.Get("client_secret"), ShouldEqual, "test-client-secret")
			So(request.form.Get("redirect_uri"), ShouldEqual, "https://app.tongji.edu.cn/wallbreakerAuth/callback.html")
			var response map[string]any
			So(json.Unmarshal(requestContext.Response.Body(), &response), ShouldBeNil)
			So(response["access_token"], ShouldEqual, "access-token")
			So(response["token_type"], ShouldEqual, "Bearer")
			So(response["expires_in"], ShouldEqual, float64(300))
			So(response["scope"], ShouldEqual, "openid user")
			So(response["refresh_token"], ShouldBeNil)
		})
	})
}

func TestTongjiExchangeTokenOptions(t *testing.T) {
	Convey("同济 OAuth 令牌交换预检", t, func() {
		Convey("登记的 callback 来源应获得最小 CORS 权限", func() {
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Origin", tongjiCallbackOrigin)

			TongjiExchangeTokenOptions(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNoContent)
			So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Origin")), ShouldEqual, tongjiCallbackOrigin)
			So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Methods")), ShouldEqual, "POST, OPTIONS")
			So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Headers")), ShouldEqual, "Content-Type")
			So(string(requestContext.Response.Header.Peek("Vary")), ShouldEqual, "Origin")
		})

		Convey("未登记来源不应获得读取令牌响应的 CORS 权限", func() {
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Origin", "https://attacker.example")

			TongjiExchangeTokenOptions(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNoContent)
			So(string(requestContext.Response.Header.Peek("Access-Control-Allow-Origin")), ShouldBeBlank)
		})
	})
}

func newTongjiJSONRequest(body, origin string) *app.RequestContext {
	requestContext := app.NewContext(0)
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Request.Header.Set("Origin", origin)
	requestContext.Request.SetBodyString(body)
	return requestContext
}

func responseError(requestContext *app.RequestContext) string {
	var response struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(requestContext.Response.Body(), &response)
	return response.Error
}

func newTongjiState(t *testing.T, tokenEndpoint string) string {
	t.Helper()
	client, err := tongjiapi.New(testTongjiConfig("https://open-platform.test/authorize", tokenEndpoint))
	if err != nil {
		t.Fatalf("create test Tongji client: %v", err)
	}
	state, err := client.CreateState()
	if err != nil {
		t.Fatalf("create test state: %v", err)
	}
	return state
}

func testTongjiConfig(authorizationEndpoint, tokenEndpoint string) tongjiapi.Config {
	return tongjiapi.Config{
		ClientID:              "test-client-id",
		ClientSecret:          "test-client-secret",
		RedirectURI:           "https://app.tongji.edu.cn/wallbreakerAuth/callback.html",
		StateSecret:           testTongjiStateSecret,
		AuthorizationEndpoint: authorizationEndpoint,
		TokenEndpoint:         tokenEndpoint,
		APIBaseURL:            "https://open-platform.test",
	}
}

func setTongjiOAuthEnv(t *testing.T, authorizationEndpoint, tokenEndpoint string) {
	t.Helper()
	t.Setenv("TONGJI_OPEN_PLATFORM_CLIENT_ID", "test-client-id")
	t.Setenv("TONGJI_OPEN_PLATFORM_CLIENT_SECRET", "test-client-secret")
	t.Setenv("TONGJI_OPEN_PLATFORM_REDIRECT_URI", "https://app.tongji.edu.cn/wallbreakerAuth/callback.html")
	t.Setenv("TONGJI_OPEN_PLATFORM_STATE_SECRET", testTongjiStateSecret)
	t.Setenv("TONGJI_OPEN_PLATFORM_AUTHORIZATION_ENDPOINT", authorizationEndpoint)
	t.Setenv("TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT", tokenEndpoint)
	t.Setenv("TONGJI_OPEN_PLATFORM_API_BASE_URL", "https://open-platform.test")
}

func clearTongjiOAuthEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TONGJI_OPEN_PLATFORM_CLIENT_ID",
		"TONGJI_OPEN_PLATFORM_CLIENT_SECRET",
		"TONGJI_OPEN_PLATFORM_REDIRECT_URI",
		"TONGJI_OPEN_PLATFORM_STATE_SECRET",
		"TONGJI_OPEN_PLATFORM_AUTHORIZATION_ENDPOINT",
		"TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT",
		"TONGJI_OPEN_PLATFORM_API_BASE_URL",
	} {
		t.Setenv(key, "")
	}
}
