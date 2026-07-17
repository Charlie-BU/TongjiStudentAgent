package handler

import (
	"context"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/agent"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type chatRequest struct {
	Message string `json:"message"`
}

// Chat invokes the initialized DeepAgent with a single user message.
func Chat(ctx context.Context, c *app.RequestContext) {
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

	response, err := agent.Chat(ctx, req.Message)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "agent invocation failed"})
		return
	}

	c.JSON(consts.StatusOK, utils.H{"message": response})
}
