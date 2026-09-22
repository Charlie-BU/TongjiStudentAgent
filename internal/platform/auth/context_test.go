package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExtractBearerToken(t *testing.T) {
	Convey("解析 Chat Bearer 授权凭据", t, func() {
		Convey("合法 Bearer 凭据应返回 token", func() {
			accessToken, err := ExtractBearerToken("  bearer test-access-token  ")

			So(err, ShouldBeNil)
			So(accessToken, ShouldEqual, "test-access-token")
		})

		Convey("缺失或格式错误的凭据应被拒绝", func() {
			for _, authorization := range []string{"", "Basic credentials", "Bearer", "Bearer token extra"} {
				_, err := ExtractBearerToken(authorization)
				So(err, ShouldNotBeNil)
			}
		})
	})
}

func TestAccessTokenContext(t *testing.T) {
	Convey("校园访问凭据请求上下文", t, func() {
		Convey("用户基础信息查询成功", func() {
			setupIdentityEndpoint(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-access-token" {
					t.Error("unexpected user credential")
				}
				fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"student-001"}]}}`)
			})

			requestContext := WithAccessToken(context.Background(), "test-access-token")
			accessToken, ok := AccessTokenFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")
			userID, ok := UserIDFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(userID, ShouldEqual, "student-001")
		})

		Convey("用户基础信息查询失败", func() {
			setupIdentityEndpoint(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })

			requestContext := WithAccessToken(context.Background(), "test-access-token")
			accessToken, ok := AccessTokenFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")
			_, ok = UserIDFromContext(requestContext)
			So(ok, ShouldBeFalse)
		})

		Convey("凭据缺失", func() {
			requestContext := WithAccessToken(context.Background(), " ")

			_, ok := AccessTokenFromContext(requestContext)
			So(ok, ShouldBeFalse)
		})
	})
}

func TestEveryRunResolvesIdentityAndClearsPreviousIdentity(t *testing.T) {
	calls := 0
	setupIdentityEndpoint(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"student-a"}]}}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	first := WithAccessToken(context.Background(), "same-user-token")
	if id, ok := UserIDFromContext(first); !ok || id != "student-a" {
		t.Fatal("missing first identity")
	}
	second := WithAccessToken(first, "same-user-token")
	if _, ok := UserIDFromContext(second); ok || calls != 2 {
		t.Fatal("identity must be resolved for each run")
	}
	if _, ok := UserIDFromContext(WithAccessToken(first, "")); ok {
		t.Fatal("anonymous run inherited identity")
	}
}

func setupIdentityEndpoint(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	for key, value := range map[string]string{"TONGJI_LOGIN_CLIENT_ID": "login", "TONGJI_LOGIN_CLIENT_SECRET": "secret", "TONGJI_OPEN_PLATFORM_REDIRECT_URI": "https://example.test", "TONGJI_OPEN_PLATFORM_STATE_SECRET": "state", "TONGJI_OPEN_PLATFORM_API_BASE_URL": server.URL} {
		t.Setenv(key, value)
	}
}
