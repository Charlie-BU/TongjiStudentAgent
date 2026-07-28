package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

type chatRequest struct {
	Message string `json:"message"`
}

// 提成包级变量，方便测试时替换 streamChat 实现
var streamChat = chat.Stream

// Chat 调用 Agent 并非流式返回结果。
func Chat(ctx context.Context, c *app.RequestContext) {
	// 若 token 合法，将 token 写入上下文，否则对 context 不做处理
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	message, ok := bindChatMessage(c)
	if !ok {
		return
	}

	response, err := chat.Chat(requestContext, message)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{"error": "agent invocation failed"})
		return
	}

	c.JSON(consts.StatusOK, utils.H{"message": response})
}

// ChatStream 以 Server-Sent Events 返回 Agent 的安全运行过程与最终文本 delta。
func ChatStream(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	// 为流式响应创建独立上下文，用于取消流
	streamContext, cancel := context.WithCancel(requestContext)
	defer cancel()
	message, ok := bindChatMessage(c)
	if !ok {
		return
	}

	c.Response.Header.Set("X-Accel-Buffering", "no")
	c.Response.SetStatusCode(consts.StatusOK)
	writer := sse.NewWriter(c) // Server-Sent Events 写入器
	var streamStopped atomic.Bool
	stopStream := func() {
		if streamStopped.CompareAndSwap(false, true) {
			cancel()
		}
	}
	_, _ = streamChat(streamContext, message, func(event agentevent.Event) {
		if streamStopped.Load() {
			return
		}
		data, err := json.Marshal(event)
		if err != nil {
			logs.CtxError(streamContext, "SSE event serialization failed")
			stopStream()
			return
		}
		if err := writer.WriteEvent(strconv.FormatInt(event.Sequence, 10), event.Type, data); err != nil {
			logs.CtxInfo(streamContext, "SSE response write failed: %v", err)
			stopStream()
		}
	})
}

// bindChatMessage 从请求体中提取用户消息。
// 若消息缺失或格式错误，返回空字符串和 false。
func bindChatMessage(c *app.RequestContext) (string, bool) {
	var req chatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return "", false
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "message is required"})
		return "", false
	}
	return req.Message, true
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
