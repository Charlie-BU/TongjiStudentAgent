package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	// your code ...
	r.GET("/", handler.HelloWorld)

	// NOTE: GET v1/ping should be provided for liveness check by FaaS Platform.
	// Status Code OK denotes that the service is healthy.
	r.GET("/v1/ping", handler.Ping)
	r.POST("/v1/agent/chat", handler.Chat)
}
