package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	chatHandler := handler.NewChatHandler(chat.DefaultService())
	r.GET("/v1/ping", handler.Ping)
	r.POST("/v1/sessions", chatHandler.CreateSession)
	r.GET("/v1/sessions", chatHandler.Sessions)
	r.DELETE("/v1/sessions", chatHandler.DeleteSession)
	r.POST("/v1/session/rename", chatHandler.RenameSession)
	r.POST("/v1/sessions/:session_id/messages", chatHandler.SessionMessageStream)
	r.GET("/v1/sessions/:session_id/messages", chatHandler.SessionMessages)
	r.GET("/v1/sessions/:session_id/task-plan", chatHandler.SessionTaskPlan)
	r.GET("/v1/tongji/oauth/authorize", handler.TongjiAuthorize)
	r.POST("/v1/tongji/oauth/token", handler.TongjiExchangeToken)
	r.GET("/v1/tongji/user/basic-info", handler.TongjiUserBasicInfo)
}
