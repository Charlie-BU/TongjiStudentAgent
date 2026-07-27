package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	platformconfig "github.com/Charlie-BU/TongjiStudent/internal/platform/config"
	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/hertz/pkg/app"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
)

func main() {
	port := flag.String("port", platformconfig.ServerPort(), "HTTP server port")
	flag.Parse()

	ctx := context.Background()

	initializeClient(ctx)

	opts := make([]hertzconfig.Option, 0, 2)
	opts = append(opts,
		hertzserver.WithHostPorts(":"+*port),
		hertzserver.WithExitWaitTime(platformconfig.GracefulTime()),
	)

	hz := hertzserver.Default(opts...)

	hz.Use(requestLoggingMiddleware)
	hz.Use(streamHeaderMiddleware)

	register(hz)

	for _, hook := range GetShutdownHooks() {
		hz.OnShutdown = append(hz.OnShutdown, hook)
	}

	hz.Spin()
}

// requestLoggingMiddleware 记录不包含请求内容的 HTTP 元数据。
func requestLoggingMiddleware(c context.Context, ctx *app.RequestContext) {
	requestID := newRequestID()
	ctx.Response.Header.Set("X-Request-ID", requestID)
	startTime := time.Now()
	logs.CtxInfo(c, "HTTP request started: request_id=%s method=%s path=%s", requestID, ctx.Request.Method(), ctx.Request.Path())
	ctx.Next(c)
	logs.CtxInfo(c, "HTTP request completed: request_id=%s method=%s path=%s status=%d duration=%s", requestID, ctx.Request.Method(), ctx.Request.Path(), ctx.Response.StatusCode(), time.Since(startTime))
}

// newRequestID 生成用于关联单次 HTTP 请求日志的随机标识。
func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return hex.EncodeToString(value)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// 流式响应头中间件，用于设置流式响应头
func streamHeaderMiddleware(c context.Context, ctx *app.RequestContext) {
	ctx.Response.Header.Set("X-Enable-Stream", "true")
	ctx.Next(c)
}
