package main

import (
	"context"
	"regexp"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
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
