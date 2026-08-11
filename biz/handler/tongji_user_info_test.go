package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	"github.com/cloudwego/hertz/pkg/app"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTongjiUserBasicInfo(t *testing.T) {
	Convey("用户基础信息接口", t, func() {
		originalNewClient := newUserBasicInfoClient
		t.Cleanup(func() { newUserBasicInfoClient = originalNewClient })

		Convey("使用 Bearer access token 查询并返回基础信息", func() {
			client := &fakeUserBasicInfoClient{info: &tongjiapi.UserBasicInfo{Name: "测试同学", UserId: "2350939", UserTypeName: "本科生"}}
			newUserBasicInfoClient = func() (userBasicInfoClient, error) { return client, nil }
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			TongjiUserBasicInfo(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(client.accessToken, ShouldEqual, "test-access-token")
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"测试同学"`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"userId":"2350939"`)
		})

		Convey("缺少或格式错误的 Bearer access token 返回 401", func() {
			for _, authorization := range []string{"", "Basic token", "Bearer"} {
				requestContext := app.NewContext(0)
				requestContext.Request.Header.Set("Authorization", authorization)
				TongjiUserBasicInfo(context.Background(), requestContext)
				So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)
			}
		})

		Convey("上游查询失败返回 502", func() {
			newUserBasicInfoClient = func() (userBasicInfoClient, error) {
				return &fakeUserBasicInfoClient{err: errors.New("upstream unavailable")}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			TongjiUserBasicInfo(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadGateway)
		})
	})
}

type fakeUserBasicInfoClient struct {
	accessToken string
	info        *tongjiapi.UserBasicInfo
	err         error
}

func (c *fakeUserBasicInfoClient) GetUserBasicInfo(_ context.Context, accessToken string) (*tongjiapi.UserBasicInfo, error) {
	c.accessToken = accessToken
	return c.info, c.err
}
