package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

type chatRequest struct {
	Message string `json:"message"` // 本轮次用户消息
}

// createSession 用于测试时替换会话创建实现。
var createSession = chat.CreateSession

// streamSession 用于测试时替换会话流式执行实现。
var streamSession = chat.StreamSession

// listSessionMessages 用于测试时替换会话历史读取实现。
var listSessionMessages = chat.ListSessionMessages

// getSessionTaskPlan 用于测试时替换会话任务计划读取实现。
var getSessionTaskPlan = chat.GetSessionTaskPlan

// CreateSession 创建与当前请求身份对应的会话。
func CreateSession(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	session, err := createSession(requestContext)
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "session service unavailable"})
		return
	}
	c.JSON(consts.StatusCreated, utils.H{"session_id": session.ID, "persistence": session.Persistence})
}

// SessionMessageStream 向指定会话提交消息并以 SSE 返回本轮执行事件。
func SessionMessageStream(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	request, ok := bindSessionMessage(c)
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "session_id is required"})
		return
	}
	streamContext, cancel := context.WithCancel(requestContext)
	defer cancel()
	c.Response.Header.Set("X-Accel-Buffering", "no")
	c.Response.SetStatusCode(consts.StatusOK)
	writer := sse.NewWriter(c)
	var streamStopped atomic.Bool
	stopStream := func() {
		if streamStopped.CompareAndSwap(false, true) {
			cancel()
		}
	}
	_, err := streamSession(streamContext, sessionID, request.Message, func(event agentevent.Event) {
		if streamStopped.Load() {
			return
		}
		event.SessionID = sessionID
		data, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			logs.CtxError(streamContext, "session SSE event serialization failed")
			stopStream()
			return
		}
		if writeErr := writer.WriteEvent(strconv.FormatInt(event.Sequence, 10), event.Type, data); writeErr != nil {
			logs.CtxInfo(streamContext, "session SSE response write failed: %v", writeErr)
			stopStream()
		}
	})
	if err != nil && !streamStopped.Load() {
		logs.CtxInfo(streamContext, "session message failed: %v", err)
	}
}

// SessionMessages 返回当前请求有权读取的会话 canonical 历史。
func SessionMessages(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "session_id is required"})
		return
	}
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 100 {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "limit must be an integer between 1 and 100"})
			return
		}
		limit = parsed
	}
	messages, err := listSessionMessages(requestContext, sessionID, limit)
	if err != nil {
		status := consts.StatusInternalServerError
		if errors.Is(err, agenticsession.ErrNotFound) {
			status = consts.StatusNotFound
		}
		c.JSON(status, utils.H{"error": "session not found"})
		return
	}
	c.JSON(consts.StatusOK, utils.H{"messages": messages})
}

// SessionTaskPlan 返回当前请求有权访问的活动任务计划。
func SessionTaskPlan(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "session_id is required"})
		return
	}
	plan, err := getSessionTaskPlan(requestContext, sessionID)
	if err != nil {
		status := consts.StatusInternalServerError
		if errors.Is(err, agenticsession.ErrNotFound) {
			status = consts.StatusNotFound
		}
		c.JSON(status, utils.H{"error": "session not found"})
		return
	}
	c.JSON(consts.StatusOK, struct {
		Plan *taskplan.TaskPlan `json:"plan"`
	}{Plan: plan})
}

// bindSessionMessage 从请求体中提取会话用户消息。
func bindSessionMessage(c *app.RequestContext) (chatRequest, bool) {
	var req chatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return chatRequest{}, false
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "message is required"})
		return chatRequest{}, false
	}
	return req, true
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
