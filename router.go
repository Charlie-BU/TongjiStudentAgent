package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	r.GET("/v1/ping", handler.Ping)
	r.POST("/v1/sessions", handler.CreateSession)
	r.GET("/v1/sessions", handler.Sessions)
	r.POST("/v1/session/rename", handler.RenameSession)
	r.POST("/v1/sessions/:session_id/messages", handler.SessionMessageStream)
	r.GET("/v1/sessions/:session_id/messages", handler.SessionMessages)
	r.GET("/v1/sessions/:session_id/task-plan", handler.SessionTaskPlan)
	r.GET("/v1/tongji/oauth/authorize", handler.TongjiAuthorize)
	r.POST("/v1/tongji/oauth/token", handler.TongjiExchangeToken)
	r.OPTIONS("/v1/tongji/oauth/token", handler.TongjiExchangeTokenOptions)
	r.GET("/v1/tongji/user/basic-info", handler.TongjiUserBasicInfo)
}
