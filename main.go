package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	logs "github.com/Charlie-BU/TongjiStudent/pkg/logging"
	"github.com/cloudwego/hertz/pkg/app"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
)

func main() {
	port := flag.String("port", GetServerPort(), "HTTP server port")
	flag.Parse()

	ctx := context.Background()

	initializeClient(ctx)

	opts := make([]config.Option, 0, 2)
	opts = append(opts,
		hertzserver.WithHostPorts(":"+*port),
		hertzserver.WithExitWaitTime(GetGracefulTime()),
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

// 请求日志中间件，用于记录请求信息和响应信息
func requestLoggingMiddleware(c context.Context, ctx *app.RequestContext) {
	startTime := time.Now()
	reqInfo := fmt.Sprintf("REQ: [Method = %s] [Path = %s] [Header = %s, Body = %s]", string(ctx.Request.Method()), string(ctx.Request.Path()), string(ctx.Request.Header.Header()), string(ctx.Request.Body()))
	logs.CtxInfo(c, reqInfo)
	ctx.Request.Header.Set("X-Bytefaas-Enable-Stream", "true")
	ctx.Next(c)
	duration := time.Since(startTime)
	respInfo := fmt.Sprintf("RESP: [Duration = %v] [Status Code = %d] [Header = %s, Body = %s]", duration, ctx.Response.StatusCode(), string(ctx.Response.Header.Header()), string(ctx.Response.Body()))
	logs.CtxInfo(c, respInfo)
}

// 流式响应头中间件，用于设置流式响应头
func streamHeaderMiddleware(c context.Context, ctx *app.RequestContext) {
	ctx.Response.Header.Set("X-Bytefaas-Enable-Stream", "true")
	ctx.Next(c)
}

// JWT转发中间件，用于将JWT令牌转发给后端服务
