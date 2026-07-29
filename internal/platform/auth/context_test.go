package auth

import (
	"context"
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
		Convey("应只从写入后的上下文读取凭据", func() {
			accessToken, ok := AccessTokenFromContext(WithAccessToken(context.Background(), "test-access-token"))
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")

			_, ok = AccessTokenFromContext(context.Background())
			So(ok, ShouldBeFalse)
		})
	})
}
