package auth

import (
	"context"
	"fmt"
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
			originalResolver := resolveUserID
			resolveUserID = func(_ context.Context, accessToken string) (string, error) {
				So(accessToken, ShouldEqual, "test-access-token")
				return "student-001", nil
			}
			t.Cleanup(func() { resolveUserID = originalResolver })

			requestContext := WithAccessToken(context.Background(), "test-access-token")
			accessToken, ok := AccessTokenFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")
			userID, ok := UserIDFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(userID, ShouldEqual, "student-001")
		})

		Convey("用户基础信息查询失败", func() {
			originalResolver := resolveUserID
			resolveUserID = func(context.Context, string) (string, error) {
				return "", fmt.Errorf("upstream unavailable")
			}
			t.Cleanup(func() { resolveUserID = originalResolver })

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
