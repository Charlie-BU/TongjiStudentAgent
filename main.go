package main

import (
	"context"
	"fmt"
	"time"

	bytedaiserver "code.byted.org/inf/bytedai-go/agent/server"
	bytedclient "code.byted.org/inf/bytedmcp/go/client"
	"code.byted.org/middleware/hertz/byted"
	"code.byted.org/middleware/hertz/pkg/app"
	hertzserver "code.byted.org/middleware/hertz/pkg/app/server"
	"code.byted.org/middleware/hertz/pkg/common/config"

	"code.byted.org/gopkg/logs/v2"
)

func main() {
	ctx := context.Background()

	initializeClient(ctx)

	byted.Init()

	opts := make([]config.Option, 0, 1)
	opts = append(opts, hertzserver.WithExitWaitTime(GetGracefulTime()))

	hz := byted.Default(opts...)

	hz.Use(requestLoggingMiddleware)
	hz.Use(streamHeaderMiddleware)
	hz.Use(jwtForwardMiddleware)

	register(hz)

	for _, hook := range GetShutdownHooks() {
		hz.OnShutdown = append(hz.OnShutdown, hook)
	}

	bytedaiserver.NewMultiA2AAgentWrapper(hz).AddAgent(ctx, "", buildA2AServerConfig(ctx))

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
func jwtForwardMiddleware(c context.Context, ctx *app.RequestContext) {
	if token := ctx.GetHeader("X-Jwt-Token"); token != nil {
		c = bytedclient.WithIdentityToken(c, string(token))
	}
	ctx.Next(c)
}
