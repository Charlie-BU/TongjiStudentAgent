package main

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHTTPResponseHeaders(t *testing.T) {
	Convey("HTTP 响应元数据", t, func() {
		requestContext := app.NewContext(0)

		requestLoggingMiddleware(context.Background(), requestContext)
		streamHeaderMiddleware(context.Background(), requestContext)

		Convey("应提供随机 Request ID 与当前流式响应头", func() {
			requestID := string(requestContext.Response.Header.Get("X-Request-ID"))

			So(regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(requestID), ShouldBeTrue)
			So(string(requestContext.Response.Header.Get("X-Enable-Stream")), ShouldEqual, "true")
			So(string(requestContext.Response.Header.Get("X-Bytefaas-Enable-Stream")), ShouldBeBlank)
		})
	})
}

func TestCORSMiddleware(t *testing.T) {
	Convey("全局 CORS 中间件", t, func() {
		t.Setenv("CORS_ALLOW_ORIGINS", `["https://app.tongji.edu.cn", "http://localhost:5173"]`)
		middleware, err := CORSMiddleware()
		So(err, ShouldBeNil)
		So(middleware, ShouldNotBeNil)

		hz := hertzserver.New()
		hz.Use(middleware)
		hz.GET("/v1/ping", func(_ context.Context, c *app.RequestContext) {
			c.Status(http.StatusOK)
		})
		hz.POST("/v1/tongji/oauth/token", func(_ context.Context, c *app.RequestContext) {
			c.Status(http.StatusOK)
		})

		Convey("允许的来源可以访问普通 API 和 OAuth token 接口", func() {
			ping := ut.PerformRequest(hz.Engine, http.MethodGet, "/v1/ping", nil,
				ut.Header{Key: "Origin", Value: "http://localhost:5173"})
			So(ping.Code, ShouldEqual, http.StatusOK)
			So(ping.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "http://localhost:5173")

			token := ut.PerformRequest(hz.Engine, http.MethodPost, "/v1/tongji/oauth/token", nil,
				ut.Header{Key: "Origin", Value: "https://app.tongji.edu.cn"})
			So(token.Code, ShouldEqual, http.StatusOK)
			So(token.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "https://app.tongji.edu.cn")
		})

		Convey("未允许来源会被中间件拒绝", func() {
			response := ut.PerformRequest(hz.Engine, http.MethodGet, "/v1/ping", nil,
				ut.Header{Key: "Origin", Value: "https://attacker.example"})
			So(response.Code, ShouldEqual, http.StatusForbidden)
			So(response.Header().Get("Access-Control-Allow-Origin"), ShouldBeBlank)
		})

		Convey("允许来源的预检会返回授权方法和 Authorization 请求头", func() {
			response := ut.PerformRequest(hz.Engine, http.MethodOptions, "/v1/tongji/oauth/token", nil,
				ut.Header{Key: "Origin", Value: "http://localhost:5173"},
				ut.Header{Key: "Access-Control-Request-Method", Value: http.MethodPost},
				ut.Header{Key: "Access-Control-Request-Headers", Value: "Authorization, Content-Type"})
			So(response.Code, ShouldEqual, http.StatusNoContent)
			So(response.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "http://localhost:5173")
			So(response.Header().Get("Access-Control-Allow-Methods"), ShouldContainSubstring, http.MethodPost)
			So(response.Header().Get("Access-Control-Allow-Headers"), ShouldContainSubstring, "Authorization")
		})
	})
}
