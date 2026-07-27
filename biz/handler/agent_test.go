package handler

import (
	"context"
	"testing"

	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatAuthorization(t *testing.T) {
	Convey("Chat 请求授权入口", t, func() {
		Convey("合法 Bearer 凭据应写入请求上下文", func() {
			requestContext := withChatAccessToken(context.Background(), "Bearer test-access-token")

			accessToken, ok := platformauth.AccessTokenFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")
		})

		Convey("缺失或格式错误的 Bearer 凭据不应阻断 Agent 调用", func() {
			for _, authorization := range []string{"", "Basic credentials", "Bearer", "Bearer token extra"} {
				requestContext := withChatAccessToken(context.Background(), authorization)
				_, ok := platformauth.AccessTokenFromContext(requestContext)
				So(ok, ShouldBeFalse)
			}
		})
	})
}
