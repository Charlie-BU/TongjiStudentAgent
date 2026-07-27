package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	r.GET("/v1/ping", handler.Ping)
	r.POST("/v1/agent/chat", handler.Chat)
	r.GET("/v1/tongji/oauth/authorize", handler.TongjiAuthorize)
	r.POST("/v1/tongji/oauth/token", handler.TongjiExchangeToken)
	r.OPTIONS("/v1/tongji/oauth/token", handler.TongjiExchangeTokenOptions)
}
