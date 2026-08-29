package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

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

type createSessionRequest struct {
	Name string `json:"name"`
}

type renameSessionRequest struct {
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
}

type deleteSessionRequest struct {
	SessionID string `json:"session_id"`
}

type sessionResponse struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Persistence  agenticsession.Persistence `json:"persistence"`
	CreatedAt    time.Time                  `json:"created_at"`
	LastActiveAt time.Time                  `json:"last_active_at"`
}

// createSessionWithName 用于测试时替换带名称的会话创建实现。
var createSessionWithName = chat.CreateSessionWithName

// listSessions 用于测试时替换会话列表读取实现。
var listSessions = chat.ListSessions

// renameSession 用于测试时替换会话重命名实现。
var renameSession = chat.RenameSession

// deleteSession 用于测试时替换会话删除实现。
var deleteSession = chat.DeleteSession

// streamSession 用于测试时替换会话流式执行实现。
var streamSession = chat.StreamSession

// listSessionMessagePage 用于测试时替换会话历史分页读取实现。
var listSessionMessagePage = chat.ListSessionMessagePage

// getSessionTaskPlan 用于测试时替换会话任务计划读取实现。
var getSessionTaskPlan = chat.GetSessionTaskPlan

// CreateSession 创建与当前请求身份对应的会话。
func CreateSession(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	request, ok := bindCreateSession(c)
	if !ok {
		return
	}
	session, err := createSessionWithName(requestContext, request.Name)
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "session service unavailable"})
		return
	}
	c.JSON(consts.StatusCreated, utils.H{"session_id": session.ID, "name": session.Name, "persistence": session.Persistence})
}

// Sessions 返回当前 AccessToken 对应用户的全部持久会话。
func Sessions(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	sessions, err := listSessions(requestContext)
	if err != nil {
		if errors.Is(err, agenticsession.ErrInvalidOwner) {
			c.JSON(consts.StatusUnauthorized, utils.H{"error": "valid access token is required"})
			return
		}
		c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "session service unavailable"})
		return
	}
	response := make([]sessionResponse, 0, len(sessions))
	for _, session := range sessions {
		response = append(response, newSessionResponse(session))
	}
	c.JSON(consts.StatusOK, utils.H{"sessions": response})
}

// RenameSession 修改当前 AccessToken 对应用户拥有的会话名称。
func RenameSession(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	request, ok := bindRenameSession(c)
	if !ok {
		return
	}
	session, err := renameSession(requestContext, request.SessionID, request.Name)
	if err != nil {
		switch {
		case errors.Is(err, agenticsession.ErrInvalidOwner):
			c.JSON(consts.StatusUnauthorized, utils.H{"error": "valid access token is required"})
		case errors.Is(err, agenticsession.ErrNotFound):
			c.JSON(consts.StatusNotFound, utils.H{"error": "session not found"})
		default:
			c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "session service unavailable"})
		}
		return
	}
	c.JSON(consts.StatusOK, newSessionResponse(session))
}

// DeleteSession 删除当前 AccessToken 对应用户拥有的会话及其关联数据。
func DeleteSession(ctx context.Context, c *app.RequestContext) {
	requestContext := withChatAccessToken(ctx, string(c.Request.Header.Get("Authorization")))
	request, ok := bindDeleteSession(c)
	if !ok {
		return
	}
	if err := deleteSession(requestContext, request.SessionID); err != nil {
		switch {
		case errors.Is(err, agenticsession.ErrInvalidOwner):
			c.JSON(consts.StatusUnauthorized, utils.H{"error": "valid access token is required"})
		case errors.Is(err, agenticsession.ErrNotFound):
			c.JSON(consts.StatusNotFound, utils.H{"error": "session not found"})
		default:
			c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "session service unavailable"})
		}
		return
	}
	c.SetStatusCode(consts.StatusNoContent)
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
	offset, snapshotSequence := 0, int64(0)
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "offset must be a non-negative integer"})
			return
		}
		offset = parsed
	}
	if rawSnapshot := strings.TrimSpace(c.Query("snapshot_sequence")); rawSnapshot != "" {
		parsed, err := strconv.ParseInt(rawSnapshot, 10, 64)
		if err != nil || parsed < 0 {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "snapshot_sequence must be a non-negative integer"})
			return
		}
		snapshotSequence = parsed
	}
	page, err := listSessionMessagePage(requestContext, sessionID, limit, offset, snapshotSequence)
	if err != nil {
		status := consts.StatusInternalServerError
		if errors.Is(err, agenticsession.ErrNotFound) {
			status = consts.StatusNotFound
		}
		c.JSON(status, utils.H{"error": "session not found"})
		return
	}
	c.JSON(consts.StatusOK, utils.H{"messages": page.Messages, "has_more": page.HasMore, "snapshot_sequence": page.SnapshotSequence})
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

// bindCreateSession 从请求体中提取创建会话请求参数。
func bindCreateSession(c *app.RequestContext) (createSessionRequest, bool) {
	if len(c.Request.Body()) == 0 {
		return createSessionRequest{Name: "New Session"}, true
	}
	var request createSessionRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return createSessionRequest{}, false
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		request.Name = "New Session"
	}
	return request, true
}

// bindRenameSession 从请求体中提取重命名会话请求参数。
func bindRenameSession(c *app.RequestContext) (renameSessionRequest, bool) {
	var request renameSessionRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return renameSessionRequest{}, false
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Name = strings.TrimSpace(request.Name)
	if request.SessionID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "session_id is required"})
		return renameSessionRequest{}, false
	}
	if request.Name == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "name is required"})
		return renameSessionRequest{}, false
	}
	return request, true
}

// bindDeleteSession 从请求体中提取待删除会话标识。
func bindDeleteSession(c *app.RequestContext) (deleteSessionRequest, bool) {
	var request deleteSessionRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "request body must be valid JSON"})
		return deleteSessionRequest{}, false
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	if request.SessionID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "session_id is required"})
		return deleteSessionRequest{}, false
	}
	return request, true
}

func newSessionResponse(session agenticsession.Session) sessionResponse {
	return sessionResponse{ID: session.ID, Name: session.Name, Persistence: session.Persistence, CreatedAt: session.CreatedAt, LastActiveAt: session.LastActiveAt}
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
