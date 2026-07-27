package tongjiapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// testRoundTripper 为客户端测试提供无网络 HTTP 响应。
type testRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试 HTTP 请求。
func (roundTrip testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type capturedRequest struct {
	method        string
	path          string
	contentType   string
	authorization string
	form          url.Values
}

func TestTongjiOpenPlatformClient(t *testing.T) {
	Convey("同济开放平台客户端成功请求", t, func() {
		requests := make(chan capturedRequest, 2)
		client := newTestClient(t, func(request *http.Request) (*http.Response, error) {
			capture := capturedRequest{
				method:        request.Method,
				path:          request.URL.Path,
				contentType:   request.Header.Get("Content-Type"),
				authorization: request.Header.Get("Authorization"),
			}
			if request.URL.Path == "/token" {
				body, _ := io.ReadAll(request.Body)
				capture.form, _ = url.ParseQuery(string(body))
			}
			requests <- capture

			switch request.URL.Path {
			case "/token":
				return jsonResponse(http.StatusOK, `{"access_token":"test-access-token","token_type":"Bearer","expires_in":300,"scope":"openid user"}`), nil
			case "/v1/dc/user/single_info":
				return jsonResponse(http.StatusOK, `{"code":"A00000","msg":"ok","data":[{"userid":"student-001"}]}`), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
			}
		})

		state, err := client.CreateState()
		So(err, ShouldBeNil)
		So(client.ValidateState(state), ShouldBeNil)

		scopes := []string{"openid", "user", "rt_onetongji_student_timetable", "dc_lib_lend_info_all"}
		authorizationURL, err := client.AuthorizationURL(state, scopes)
		So(err, ShouldBeNil)
		parsedURL, err := url.Parse(authorizationURL)
		So(err, ShouldBeNil)
		So(parsedURL.Query().Get("response_type"), ShouldEqual, "code")
		So(parsedURL.Query().Get("redirect_uri"), ShouldEqual, "https://app.tongji.edu.cn/wallbreakerAuth/callback.html")
		So(slices.Equal(strings.Fields(parsedURL.Query().Get("scope")), scopes), ShouldBeTrue)

		token, err := client.ExchangeAuthorizationCode(context.Background(), "code-from-callback")
		So(err, ShouldBeNil)
		So(token.AccessToken, ShouldEqual, "test-access-token")
		response, err := client.GetSingleInfo(context.Background(), token.AccessToken)
		So(err, ShouldBeNil)
		So(string(response.Data), ShouldEqual, `[{"userid":"student-001"}]`)

		tokenRequest := <-requests
		So(tokenRequest.method, ShouldEqual, http.MethodPost)
		So(tokenRequest.path, ShouldEqual, "/token")
		So(tokenRequest.contentType, ShouldEqual, "application/x-www-form-urlencoded")
		So(tokenRequest.form.Get("grant_type"), ShouldEqual, "authorization_code")
		So(tokenRequest.form.Get("code"), ShouldEqual, "code-from-callback")
		So(tokenRequest.form.Get("client_id"), ShouldEqual, "client-id")
		So(tokenRequest.form.Get("client_secret"), ShouldEqual, "client-secret")
		So(tokenRequest.form.Get("redirect_uri"), ShouldEqual, "https://app.tongji.edu.cn/wallbreakerAuth/callback.html")
		apiRequest := <-requests
		So(apiRequest.method, ShouldEqual, http.MethodGet)
		So(apiRequest.path, ShouldEqual, "/v1/dc/user/single_info")
		So(apiRequest.authorization, ShouldEqual, "Bearer test-access-token")
	})
}

func TestTongjiOpenPlatformClientRejectsInvalidInput(t *testing.T) {
	Convey("同济开放平台客户端拒绝无效 OAuth 输入", t, func() {
		client := newTestClient(t, func(*http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{}`), nil
		})

		Convey("伪造或格式错误的 state", func() {
			So(client.ValidateState("forged.state"), ShouldNotBeNil)
			So(client.ValidateState("not-a-state"), ShouldNotBeNil)
		})

		Convey("空授权码和空 access token", func() {
			_, err := client.ExchangeAuthorizationCode(context.Background(), " ")
			So(err, ShouldNotBeNil)
			_, err = client.GetSingleInfo(context.Background(), " ")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestTongjiOpenPlatformClientHandlesUpstreamFailure(t *testing.T) {
	Convey("同济开放平台客户端处理上游异常", t, func() {
		tests := []struct {
			name        string
			statusCode  int
			body        string
			exchange    bool
			wantErrPart string
		}{
			{name: "令牌端点 HTTP 失败", statusCode: http.StatusBadGateway, body: `{"error":"unavailable"}`, exchange: true, wantErrPart: "HTTP 502"},
			{name: "令牌端点缺少 access token", statusCode: http.StatusOK, body: `{}`, exchange: true, wantErrPart: "access_token"},
			{name: "数据接口业务失败", statusCode: http.StatusOK, body: `{"code":"B00001","msg":"forbidden"}`, wantErrPart: "code B00001"},
		}

		for _, test := range tests {
			test := test
			Convey(test.name, func() {
				client := newTestClient(t, func(*http.Request) (*http.Response, error) {
					return jsonResponse(test.statusCode, test.body), nil
				})

				var err error
				if test.exchange {
					_, err = client.ExchangeAuthorizationCode(context.Background(), "code")
				} else {
					_, err = client.GetSingleInfo(context.Background(), "access-token")
				}

				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, test.wantErrPart)
			})
		}
	})
}

func newTestClient(t *testing.T, transport testRoundTripper) *Client {
	t.Helper()
	client, err := New(Config{
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		RedirectURI:           "https://app.tongji.edu.cn/wallbreakerAuth/callback.html",
		StateSecret:           "state-secret",
		AuthorizationEndpoint: "https://open-platform.test/authorize",
		TokenEndpoint:         "https://open-platform.test/token",
		APIBaseURL:            "https://open-platform.test",
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.httpClient = &http.Client{Transport: transport}
	return client
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": {"application/json"}},
	}
}
