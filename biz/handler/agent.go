package handler

import (
	"context"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type chatRequest struct {
	Message string `json:"message"`
}

// Chat invokes the initialized DeepAgent with a single user message.
func Chat(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))

	var req chatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "message is required"})
		return
	}

	response, err := chat.Chat(requestContext, req.Message)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "agent invocation failed"})
		return
	}

	c.JSON(consts.StatusOK, utils.H{"message": response})
}

// withChatAccessToken 将格式正确的 Chat Bearer 凭据写入调用上下文。
// 凭据缺失或格式错误时，仍允许 Agent 调用继续执行。
func withChatAccessToken(ctx context.Context, authorization string) context.Context {
	accessToken, err := platformauth.ExtractBearerToken(authorization)
	if err != nil {
		return ctx
	}
	return platformauth.WithAccessToken(ctx, accessToken)
}
