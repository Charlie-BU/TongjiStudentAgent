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
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/hertz/pkg/app"
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

func TestChatStream(t *testing.T) {
	Convey("SSE Chat 接口", t, func() {
		originalStreamChat := streamChat
		t.Cleanup(func() { streamChat = originalStreamChat })

		Convey("应返回可解析的 SSE 事件及流式响应头", func() {
			streamChat = func(_ context.Context, message string, send func(agentevent.Event)) (string, error) {
				if message != "现在几点？" {
					return "", errors.New("unexpected message")
				}
				send(agentevent.Event{Type: agentevent.RunStarted, RunID: "run-test", Sequence: 1, OccurredAt: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)})
				send(agentevent.Event{Type: agentevent.RunFailed, RunID: "run-test", Sequence: 2, OccurredAt: time.Date(2026, 7, 28, 0, 0, 1, 0, time.UTC), Data: map[string]string{"code": "agent_execution_failed"}})
				return "", errors.New("agent failed")
			}
			requestContext := newAgentJSONRequest(`{"message":"现在几点？"}`)
			writer := &testSSEWriter{}
			requestContext.Response.HijackWriter(writer)

			ChatStream(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusOK)
			So(string(requestContext.Response.Header.ContentType()), ShouldEqual, "text/event-stream; charset=utf-8")
			So(string(requestContext.Response.Header.Peek("Cache-Control")), ShouldEqual, "no-cache")
			So(string(requestContext.Response.Header.Peek("X-Accel-Buffering")), ShouldEqual, "no")
			So(writer.String(), ShouldContainSubstring, "event: run.started")
			So(writer.String(), ShouldContainSubstring, "event: run.failed")
			So(writer.String(), ShouldContainSubstring, `"run_id":"run-test"`)
		})

		Convey("SSE 写入失败时应取消 Agent 执行", func() {
			canceled := false
			streamChat = func(ctx context.Context, _ string, send func(agentevent.Event)) (string, error) {
				send(agentevent.Event{Type: agentevent.RunStarted, Sequence: 1})
				select {
				case <-ctx.Done():
					canceled = true
				default:
					return "", errors.New("stream context was not canceled")
				}
				send(agentevent.Event{Type: agentevent.RunFailed, Sequence: 2})
				return "", ctx.Err()
			}
			requestContext := newAgentJSONRequest(`{"message":"现在几点？"}`)
			writer := &testSSEWriter{writeErr: errors.New("client disconnected")}
			requestContext.Response.HijackWriter(writer)

			ChatStream(context.Background(), requestContext)

			So(canceled, ShouldBeTrue)
			So(writer.writeCount, ShouldEqual, 1)
		})

		Convey("请求体非法时应返回 400 JSON 错误", func() {
			requestContext := newAgentJSONRequest(`{"message":`)

			ChatStream(context.Background(), requestContext)

			So(requestContext.Response.StatusCode(), ShouldEqual, http.StatusBadRequest)
			var response struct {
				Error string `json:"error"`
			}
			So(json.Unmarshal(requestContext.Response.Body(), &response), ShouldBeNil)
			So(response.Error, ShouldEqual, "request body must be valid JSON")
		})
	})
}

type testSSEWriter struct {
	bytes.Buffer
	writeErr   error
	writeCount int
}

func (w *testSSEWriter) Write(data []byte) (int, error) {
	w.writeCount++
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.Buffer.Write(data)
}

func (w *testSSEWriter) Flush() error    { return nil }
func (w *testSSEWriter) Finalize() error { return nil }

func newAgentJSONRequest(body string) *app.RequestContext {
	requestContext := app.NewContext(0)
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Request.SetBodyString(body)
	return requestContext
}
