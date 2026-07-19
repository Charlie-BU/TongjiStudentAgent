package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	r.GET("/v1/ping", handler.Ping)
	r.POST("/v1/agent/chat", handler.Chat)
}
