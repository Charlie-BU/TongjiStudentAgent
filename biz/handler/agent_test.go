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
	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
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
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("读取会话任务计划接口", t, func() {

		Convey("会返回当前计划快照", func() {
			service.getSessionTaskPlan = func(_ context.Context, sessionID string) (*taskplan.TaskPlan, error) {
				So(sessionID, ShouldEqual, "ses-001")
				return &taskplan.TaskPlan{SessionID: sessionID, Revision: 2, Tasks: []taskplan.TaskItem{{ID: "step1", Desc: "查询成绩", Status: taskplan.TaskStatusDone}}}, nil
			}
			requestContext := newSessionRequest("ses-001")

			handler.SessionTaskPlan(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"revision":2`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"status":"done"`)
		})

		Convey("无计划时返回空 plan，会话不存在时返回 404", func() {
			service.getSessionTaskPlan = func(context.Context, string) (*taskplan.TaskPlan, error) { return nil, nil }
			requestContext := newSessionRequest("ses-001")
			handler.SessionTaskPlan(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"plan":null`)

			service.getSessionTaskPlan = func(context.Context, string) (*taskplan.TaskPlan, error) { return nil, agenticsession.ErrNotFound }
			requestContext = newSessionRequest("ses-001")
			handler.SessionTaskPlan(context.Background(), requestContext)
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
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("创建会话接口", t, func() {

		Convey("会返回创建的会话标识与持久化类型", func() {
			service.createSession = func(ctx context.Context, name string) (agenticsession.Session, error) {
				accessToken, ok := platformauth.AccessTokenFromContext(ctx)
				So(ok, ShouldBeTrue)
				So(accessToken, ShouldEqual, "test-access-token")
				So(name, ShouldEqual, "New Session")
				return agenticsession.Session{ID: "ses-001", Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			handler.CreateSession(context.Background(), requestContext)

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
			service.createSession = func(_ context.Context, name string) (agenticsession.Session, error) {
				So(name, ShouldEqual, "成绩查询")
				return agenticsession.Session{ID: "ses-001", Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := newAgentJSONRequest(`{"name":" 成绩查询 "}`)

			handler.CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusCreated)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"成绩查询"`)
		})

		Convey("请求体不是合法 JSON 时返回 400", func() {
			requestContext := newAgentJSONRequest(`{"name":`)

			handler.CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
		})

		Convey("服务不可用时返回 503", func() {
			service.createSession = func(context.Context, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, errors.New("store unavailable")
			}
			requestContext := app.NewContext(0)

			handler.CreateSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusServiceUnavailable)
		})
	})
}

func TestSessionsAndRenameSession(t *testing.T) {
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("会话列表与重命名接口", t, func() {

		Convey("会根据当前访问凭据返回全部会话", func() {
			service.listSessions = func(ctx context.Context) ([]agenticsession.Session, error) {
				accessToken, ok := platformauth.AccessTokenFromContext(ctx)
				So(ok, ShouldBeTrue)
				So(accessToken, ShouldEqual, "test-access-token")
				return []agenticsession.Session{{ID: "ses-001", Name: "成绩查询", Persistence: agenticsession.PersistenceDurable}}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			handler.Sessions(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"id":"ses-001"`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"成绩查询"`)
		})

		Convey("会校验重命名输入并返回更新后的会话", func() {
			service.renameSession = func(_ context.Context, sessionID, name string) (agenticsession.Session, error) {
				So(sessionID, ShouldEqual, "ses-001")
				So(name, ShouldEqual, "新名称")
				return agenticsession.Session{ID: sessionID, Name: name, Persistence: agenticsession.PersistenceDurable}, nil
			}
			requestContext := app.NewContext(0)
			requestContext.Request.Header.Set("Content-Type", "application/json")
			requestContext.Request.SetBodyString(`{"session_id":"ses-001","name":" 新名称 "}`)

			handler.RenameSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"name":"新名称"`)
		})

		Convey("无有效身份时列表与重命名均返回 401", func() {
			service.listSessions = func(context.Context) ([]agenticsession.Session, error) { return nil, agenticsession.ErrInvalidOwner }
			requestContext := app.NewContext(0)
			handler.Sessions(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)

			service.renameSession = func(context.Context, string, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, agenticsession.ErrInvalidOwner
			}
			requestContext = newAgentJSONRequest(`{"session_id":"ses-001","name":"新名称"}`)
			handler.RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)
		})

		Convey("重命名非本人或不存在的会话返回 404，缺少字段返回 400", func() {
			service.renameSession = func(context.Context, string, string) (agenticsession.Session, error) {
				return agenticsession.Session{}, agenticsession.ErrNotFound
			}
			requestContext := newAgentJSONRequest(`{"session_id":"ses-other","name":"新名称"}`)
			handler.RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)

			requestContext = newAgentJSONRequest(`{"session_id":"ses-001"}`)
			handler.RenameSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
		})
	})
}

func TestDeleteSession(t *testing.T) {
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("删除会话接口", t, func() {

		Convey("会校验会话标识并删除当前用户的会话", func() {
			service.deleteSession = func(ctx context.Context, sessionID string) error {
				accessToken, ok := platformauth.AccessTokenFromContext(ctx)
				So(ok, ShouldBeTrue)
				So(accessToken, ShouldEqual, "test-access-token")
				So(sessionID, ShouldEqual, "ses-001")
				return nil
			}
			requestContext := newAgentJSONRequest(`{"session_id":" ses-001 "}`)
			requestContext.Request.Header.Set("Authorization", "Bearer test-access-token")

			handler.DeleteSession(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNoContent)
		})

		Convey("未认证、会话不存在及缺少会话标识时返回对应状态", func() {
			service.deleteSession = func(context.Context, string) error { return agenticsession.ErrInvalidOwner }
			requestContext := newAgentJSONRequest(`{"session_id":"ses-001"}`)
			handler.DeleteSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusUnauthorized)

			service.deleteSession = func(context.Context, string) error { return agenticsession.ErrNotFound }
			requestContext = newAgentJSONRequest(`{"session_id":"ses-001"}`)
			handler.DeleteSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)

			requestContext = newAgentJSONRequest(`{}`)
			handler.DeleteSession(context.Background(), requestContext)
			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
		})
	})
}

func TestSessionMessages(t *testing.T) {
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("读取会话历史接口", t, func() {

		Convey("会转发分页快照参数，并返回分页元数据", func() {
			service.listSessionMessages = func(_ context.Context, sessionID string, limit, offset int, snapshotSequence int64) (agenticsession.MessagePage, error) {
				So(sessionID, ShouldEqual, "ses-001")
				So(limit, ShouldEqual, 2)
				So(offset, ShouldEqual, 4)
				So(snapshotSequence, ShouldEqual, int64(10))
				return agenticsession.MessagePage{Messages: []agenticsession.Message{{ID: "msg-001", SessionID: sessionID, Role: agenticsession.MessageRoleUser, Content: "你好"}}, HasMore: true, SnapshotSequence: 10}, nil
			}
			requestContext := newSessionRequest("ses-001")
			requestContext.Request.SetRequestURI("/v1/sessions/ses-001/messages?limit=2&offset=4&snapshot_sequence=10")

			handler.SessionMessages(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"content":"你好"`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"has_more":true`)
			So(string(requestContext.Response.Body()), ShouldContainSubstring, `"snapshot_sequence":10`)
		})

		Convey("会话不存在时返回 404", func() {
			service.listSessionMessages = func(context.Context, string, int, int, int64) (agenticsession.MessagePage, error) {
				return agenticsession.MessagePage{}, agenticsession.ErrNotFound
			}
			requestContext := newSessionRequest("ses-001")

			handler.SessionMessages(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusNotFound)
		})
	})
}

func TestSessionMessageStream(t *testing.T) {
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("提交会话消息接口", t, func() {
		service.validateModelTier = func(string) error { return nil }

		Convey("会将会话标识写入 SSE 事件", func() {
			service.streamSession = func(_ context.Context, sessionID, message string, send func(agentevent.Event), tier string) (string, error) {
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

			handler.SessionMessageStream(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(writer.String(), ShouldContainSubstring, "event: run.started")
			So(writer.String(), ShouldContainSubstring, `"session_id":"anon-001"`)
		})

		Convey("缺少会话标识时返回 400 且不调用服务", func() {
			called := false
			service.streamSession = func(context.Context, string, string, func(agentevent.Event), string) (string, error) {
				called = true
				return "", nil
			}
			requestContext := newAgentJSONRequest(`{"message":"你好"}`)

			handler.SessionMessageStream(context.Background(), requestContext)

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

func TestMessageModelTierValidation(t *testing.T) {
	service := &fakeChatService{}
	handler := NewChatHandler(service)
	Convey("HTTP 层默认档位和显式档位校验", t, func() {
		for _, tc := range []struct {
			body, tier string
			status     int
		}{
			{`{"message":"hi"}`, "lite", 200},
			{`{"message":"hi","model_tier":"lite"}`, "lite", 200},
			{`{"message":"hi","model_tier":"pro"}`, "pro", 200},
			{`{"message":"hi","model_tier":"max"}`, "max", 200},
			{`{"message":"hi","model_tier":"PRO"}`, "", 400},
			{`{"message":"hi","model_tier":""}`, "", 400},
			{`{"message":"hi","model_tier":"other"}`, "", 400},
			{`{"message":"hi","model_tier":null}`, "", 400},
			{`{"message":"hi","model_tier":12}`, "", 400},
		} {
			Convey(tc.body, func() {
				service.validateModelTier = func(tier string) error {
					switch tier {
					case "lite", "pro", "max":
						return nil
					}
					return chat.ErrInvalidModelTier
				}
				called := false
				service.streamSession = func(_ context.Context, _, _ string, _ func(agentevent.Event), tier string) (string, error) {
					called = true
					So(tier, ShouldEqual, tc.tier)
					return "", nil
				}
				c := newSessionRequest("anon-001")
				c.Request.Header.Set("Content-Type", "application/json")
				c.Request.SetBodyString(tc.body)
				handler.SessionMessageStream(context.Background(), c)
				So(c.Response.StatusCode(), ShouldEqual, tc.status)
				So(called, ShouldEqual, tc.status == 200)
			})
		}
		service.validateModelTier = func(string) error { return chat.ErrModelTierUnavailable }
		called := false
		service.streamSession = func(context.Context, string, string, func(agentevent.Event), string) (string, error) {
			called = true
			return "", nil
		}
		c := newSessionRequest("anon-001")
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.SetBodyString(`{"message":"hi","model_tier":"pro"}`)
		handler.SessionMessageStream(context.Background(), c)
		So(c.Response.StatusCode(), ShouldEqual, 503)
		So(called, ShouldBeFalse)
	})
}

// Each test owns its service double; no production function is replaced.
type fakeChatService struct {
	createSession       func(ctx context.Context, name string) (agenticsession.Session, error)
	listSessions        func(ctx context.Context) ([]agenticsession.Session, error)
	renameSession       func(ctx context.Context, sessionID, name string) (agenticsession.Session, error)
	deleteSession       func(ctx context.Context, sessionID string) error
	streamSession       func(ctx context.Context, sessionID, query string, send func(agentevent.Event), tier string) (string, error)
	validateModelTier   func(tier string) error
	listSessionMessages func(ctx context.Context, sessionID string, limit, offset int, snapshotSequence int64) (agenticsession.MessagePage, error)
	getSessionTaskPlan  func(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error)
}

func (s *fakeChatService) CreateSession(ctx context.Context, name string) (agenticsession.Session, error) {
	return s.createSession(ctx, name)
}
func (s *fakeChatService) ListSessions(ctx context.Context) ([]agenticsession.Session, error) {
	return s.listSessions(ctx)
}
func (s *fakeChatService) RenameSession(ctx context.Context, sessionID, name string) (agenticsession.Session, error) {
	return s.renameSession(ctx, sessionID, name)
}
func (s *fakeChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return s.deleteSession(ctx, sessionID)
}
func (s *fakeChatService) StreamSession(ctx context.Context, sessionID, query string, send func(agentevent.Event), tier string) (string, error) {
	return s.streamSession(ctx, sessionID, query, send, tier)
}
func (s *fakeChatService) ValidateModelTier(tier string) error { return s.validateModelTier(tier) }
func (s *fakeChatService) ListSessionMessages(ctx context.Context, sessionID string, limit, offset int, snapshotSequence int64) (agenticsession.MessagePage, error) {
	return s.listSessionMessages(ctx, sessionID, limit, offset, snapshotSequence)
}
func (s *fakeChatService) GetSessionTaskPlan(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error) {
	return s.getSessionTaskPlan(ctx, sessionID)
}
