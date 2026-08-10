package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatAuthorization(t *testing.T) {
	Convey("Chat 请求授权入口", t, func() {
		Convey("合法 Bearer 凭据应写入请求上下文", func() {
			requestContext := withChatAccessToken(context.Background(), "Bearer test-access-token")

			accessToken, ok := platformauth.AccessTokenFromContext(requestContext)
			So(ok, ShouldBeTrue)
			So(accessToken, ShouldEqual, "test-access-token")
		})

		Convey("缺失或格式错误的 Bearer 凭据不应阻断 Agent 调用", func() {
			for _, authorization := range []string{"", "Basic credentials", "Bearer", "Bearer token extra"} {
				requestContext := withChatAccessToken(context.Background(), authorization)
				_, ok := platformauth.AccessTokenFromContext(requestContext)
				So(ok, ShouldBeFalse)
			}
		})
	})
}

func TestSessionTaskPlan(t *testing.T) {
	Convey("读取会话任务计划接口", t, func() {
		originalGetSessionTaskPlan := getSessionTaskPlan
		t.Cleanup(func() { getSessionTaskPlan = originalGetSessionTaskPlan })

		Convey("会返回当前计划快照", func() {
			getSessionTaskPlan = func(_ context.Context, sessionID string) (*taskplan.TaskPlan, error) {
				So(sessionID, ShouldEqual, "ses-001")
				return &taskplan.TaskPlan{SessionID: sessionID, Revision: 2, Tasks: []taskplan.TaskItem{{ID: "step1", Desc: "查询成绩", Status: taskplan.TaskStatusDone}}}, nil
			}
			requestContext := newSessionRequest("ses-001")

			SessionTaskPlan(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"revision":2`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"status":"done"`)
		})

		Convey("无计划时返回空 plan，会话不存在时返回 404", func() {
			getSessionTaskPlan = func(context.Context, string) (*taskplan.TaskPlan, error) { return nil, nil }
			requestContext := newSessionRequest("ses-001")
			SessionTaskPlan(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"plan":null`)

			getSessionTaskPlan = func(context.Context, string) (*taskplan.TaskPlan, error) { return nil, agenticsession.ErrNotFound }
			requestContext = newSessionRequest("ses-001")
			SessionTaskPlan(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)
		})
	})
}

func TestBindSessionMessage(t *testing.T) {
	Convey("会话消息请求", t, func() {
		Convey("要求消息", func() {
			requestContext := newAgentJSONRequest(`{"message":"你好"}`)
			request, ok := bindSessionMessage(requestContext)

			So(ok, ShouldBeTrue)
			So(request.Message, ShouldEqual, "你好")
		})
	})
}

func TestCreateSession(t *testing.T) {
	Convey("创建会话接口", t, func() {
		originalCreateSessionWithName := createSessionWithName
		t.Cleanup(func() { createSessionWithName = originalCreateSessionWithName })

		Convey("会返回创建的会话标识与持久化类型", func() {
			createSessionWithName = func(ctx context.Context, name string) (agenticsession.Session, error) {
				accessToken, ok := platformauth.AccessTokenFromContext(ctx)
				So(ok, ShouldBeTrue)
				So(accessToken, ShouldEqual, "test-access-token")
				So(name, ShouldEqual, "New Session")
				return agenticsession.Session{ID: "ses-001", Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusCreated)
			var response struct {
				SessionID   string                     `json:"session_id"`
				Persistence agenticsession.Persistence `json:"persistence"`
			}
			So(json.Unmarshal(requestContext.Response.Body(), &response), ShouldBeNil)
			So(response.SessionID, ShouldEqual, "ses-001")
			So(response.Persistence, ShouldEqual, agenticsession.PersistenceDurable)
		})

		Convey("会保留请求体中的会话名称", func() {
			createSessionWithName = func(_ context.Context, name string) (agenticsession.Session, error) {
				So(name, ShouldEqual, "成绩查询")
				return agenticsession.Session{ID: "ses-001", Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := newAgentJSONRequest(`{"name":" 成绩查询 "}`)

			CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusCreated)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"成绩查询"`)
		})

		Convey("请求体不是合法 JSON 时返回 400", func() {
			requestContext := newAgentJSONRequest(`{"name":`)

			CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
		})

		Convey("服务不可用时返回 503", func() {
			createSessionWithName = func(context.Context, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, errors.New("store unavailable")
			}
			requestContext := app.NewContext(0)

			CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusServiceUnavailable)
		})
	})
}

func TestSessionsAndRenameSession(t *testing.T) {
	Convey("会话列表与重命名接口", t, func() {
		originalListSessions := listSessions
		originalRenameSession := renameSession
		t.Cleanup(func() {
			listSessions = originalListSessions
			renameSession = originalRenameSession
		})

		Convey("会根据当前访问凭据返回全部会话", func() {
			listSessions = func(ctx context.Context) ([]agenticsession.Session, error) {
				accessToken, ok := platformauth.AccessTokenFromContext(ctx)
				So(ok, ShouldBeTrue)
				So(accessToken, ShouldEqual, "test-access-token")
				return []agenticsession.Session{{ID: "ses-001", Name: "成绩查询", Persistence: agenticsession.PersistenceDurable}}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			Sessions(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"id":"ses-001"`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"成绩查询"`)
		})

		Convey("会校验重命名输入并返回更新后的会话", func() {
			renameSession = func(_ context.Context, sessionID, name string) (agenticsession.Session, error) {
				So(sessionID, ShouldEqual, "ses-001")
				So(name, ShouldEqual, "新名称")
				return agenticsession.Session{ID: sessionID, Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Content-Type", "application/json")
			requestContext.Request.SetBodyString(`{"session_id":"ses-001","name":" 新名称 "}`)

			RenameSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"新名称"`)
		})

		Convey("无有效身份时列表与重命名均返回 401", func() {
			listSessions = func(context.Context) ([]agenticsession.Session, error) { return nil, agenticsession.ErrInvalidOwner }
			requestContext := app.NewContext(0)
			Sessions(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)

			renameSession = func(context.Context, string, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, agenticsession.ErrInvalidOwner
			}
			requestContext = newAgentJSONRequest(`{"session_id":"ses-001","name":"新名称"}`)
			RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)
		})

		Convey("重命名非本人或不存在的会话返回 404，缺少字段返回 400", func() {
			renameSession = func(context.Context, string, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, agenticsession.ErrNotFound
			}
			requestContext := newAgentJSONRequest(`{"session_id":"ses-other","name":"新名称"}`)
			RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)

			requestContext = newAgentJSONRequest(`{"session_id":"ses-001"}`)
			RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
		})
	})
}

func TestSessionMessages(t *testing.T) {
	Convey("读取会话历史接口", t, func() {
		originalListSessionMessages := listSessionMessages
		t.Cleanup(func() { listSessionMessages = originalListSessionMessages })

		Convey("会转发会话标识和分页上限，并返回历史", func() {
			listSessionMessages = func(_ context.Context, sessionID string, limit int) ([]agenticsession.Message, error) {
				So(sessionID, ShouldEqual, "ses-001")
				So(limit, ShouldEqual, 2)
				return []agenticsession.Message{{ID: "msg-001", SessionID: sessionID, Role: agenticsession.MessageRoleUser, Content: "你好"}}, nil
			}
			requestContext := newSessionRequest("ses-001")
			requestContext.Request.SetRequestURI("/v1/sessions/ses-001/messages?limit=2")

			SessionMessages(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"content":"你好"`)
		})

		Convey("会话不存在时返回 404", func() {
			listSessionMessages = func(context.Context, string, int) ([]agenticsession.Message, error) {
				return nil, agenticsession.ErrNotFound
			}
			requestContext := newSessionRequest("ses-001")

			SessionMessages(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)
		})
	})
}

func TestSessionMessageStream(t *testing.T) {
	Convey("提交会话消息接口", t, func() {
		originalStreamSession := streamSession
		t.Cleanup(func() { streamSession = originalStreamSession })

		Convey("会将会话标识写入 SSE 事件", func() {
			streamSession = func(_ context.Context, sessionID, message string, send func(agentevent.Event)) (string, error) {
				So(sessionID, ShouldEqual, "anon-001")
				So(message, ShouldEqual, "现在几点？")
				send(agentevent.Event{Type: agentevent.RunStarted, RunID: "run-test", Sequence: 1, OccurredAt: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)})
				return "", nil
			}
			requestContext := newSessionRequest("anon-001")
			requestContext.Request.Header.Set("Content-Type", "application/json")
			requestContext.Request.SetBodyString(`{"message":"现在几点？"}`)
			writer := &testSSEWriter{}
			requestContext.Response.HijackWriter(writer)

			SessionMessageStream(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(writer.String(), ShouldContainSubstring, "event: run.started")
			So(writer.String(), ShouldContainSubstring, `"session_id":"anon-001"`)
		})

		Convey("缺少会话标识时返回 400 且不调用服务", func() {
			called := false
			streamSession = func(context.Context, string, string, func(agentevent.Event)) (string, error) {
				called = true
				return "", nil
			}
			requestContext := newAgentJSONRequest(`{"message":"你好"}`)

			SessionMessageStream(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
			So(called, ShouldBeFalse)
		})
	})
}

func newAgentJSONRequest(body string) *app.RequestContext {
	requestContext := app.NewContext(0)
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Request.SetBodyString(body)
	return requestContext
}

func newSessionRequest(sessionID string) *app.RequestContext {
	requestContext := app.NewContext(0)
	requestContext.Params = param.Params{{Key: "session_id", Value: sessionID}}
	return requestContext
}

type testSSEWriter struct {
	bytes.Buffer
}

func (w *testSSEWriter) Flush() error    { return nil }
func (w *testSSEWriter) Finalize() error { return nil }
