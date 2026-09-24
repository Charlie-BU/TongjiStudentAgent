package main

import (
	"github.com/Charlie-BU/TongjiStudent/biz/handler"
	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	a2atransport "github.com/Charlie-BU/TongjiStudent/internal/transport/a2a"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	chatHandler := handler.NewChatHandler(chat.DefaultService())
	a2aHandler, err := a2atransport.New(chat.DefaultService(), a2atransport.Config{})
	if err != nil {
		panic(err)
	}
	r.Any("/a2a/*path", adaptor.HertzHandler(a2aHandler))
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
