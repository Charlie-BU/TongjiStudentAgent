package openroutermodel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

// toolResponse 构造包含不透明推理数据的工具响应。
func toolResponse() response {
	return response{ID: "resp-1", Status: "completed", Output: []json.RawMessage{
		json.RawMessage(`{"type":"reasoning","id":"rs-1","encrypted_content":"opaque","summary":[{"text":"检查课表"}]}`),
		json.RawMessage(fmt.Sprintf(`{"type":"function_call","call_id":"call-1","name":%q,"arguments":"{}"}`, wireToolName("tongji.student.timetable"))),
	}}
}

// textResponse 构造文本与缓存用量响应。
func textResponse(text string) response {
	raw, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}})
	cached := 1024
	u := &usage{Input: 2048, Output: 2, Total: 2050}
	u.InputDetails.Cached = &cached
	return response{ID: "resp-2", Status: "completed", Output: []json.RawMessage{raw}, Usage: u}
}

// sse 编码单个合成流事件。
func sse(v any) string { b, _ := json.Marshal(v); return "data: " + string(b) + "\n\n" }

func TestChatModel_ResponsesRoundTrip(t *testing.T) {
	Convey("完整 Responses 工具与推理往返", t, func() {
		var requests []map[string]any
		var paths, auth []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			requests = append(requests, req)
			paths = append(paths, r.URL.Path)
			auth = append(auth, r.Header.Get("Authorization"))
			result := toolResponse()
			if len(requests) > 1 {
				result = textResponse("明天有课")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(map[string]any{"type": "response.completed", "response": result}))
		}))
		defer server.Close()
		m, err := newModel(config{APIKey: "test-key", Model: "vendor/lite", BaseURL: server.URL + "/api/v1", HTTPClient: server.Client()})
		So(err, ShouldBeNil)
		tools := model.WithTools([]*schema.ToolInfo{{Name: "tongji.student.timetable", Desc: "课表"}})
		ctx := modelmeta.WithSession(context.Background(), "session-a")
		history := []*schema.Message{schema.SystemMessage("固定规则"), schema.UserMessage("明天有课吗")}
		first, err := m.Generate(ctx, history, tools)
		So(err, ShouldBeNil)
		So(first.ToolCalls[0].Function.Name, ShouldEqual, "tongji.student.timetable")
		So(first.ToolCalls[0].ID, ShouldEqual, "call-1")
		So(first.ReasoningContent, ShouldEqual, "检查课表")
		history = append(history, first, schema.ToolMessage(`{"courses":["数学"]}`, "call-1"))
		second, err := m.Generate(ctx, history, tools)
		So(err, ShouldBeNil)
		So(second.Content, ShouldEqual, "明天有课")
		So(paths, ShouldResemble, []string{"/api/v1/responses", "/api/v1/responses"})
		So(auth, ShouldResemble, []string{"Bearer test-key", "Bearer test-key"})
		for _, req := range requests {
			So(req["model"], ShouldEqual, "vendor/lite")
			So(req["session_id"], ShouldEqual, "session-a")
			So(req["store"], ShouldEqual, false)
			So(req, ShouldNotContainKey, "previous_response_id")
			So(req["tools"], ShouldHaveLength, 1)
		}
		data, _ := json.Marshal(requests[1]["input"])
		So(string(data), ShouldContainSubstring, `"encrypted_content":"opaque"`)
		So(strings.Count(string(data), `"type":"function_call"`), ShouldEqual, 1)
		So(string(data), ShouldContainSubstring, `"type":"function_call_output"`)
	})
}

func TestChatModel_Stream(t *testing.T) {
	Convey("流完成校验与文本去重", t, func() {
		delta := sse(map[string]any{"type": "response.output_text.delta", "delta": "你"})
		completed := sse(map[string]any{"type": "response.completed", "response": textResponse("你好")})
		fixtures := []struct {
			name, body string
			failed     bool
		}{
			{"正常", delta + completed, false}, {"断流", delta + "data: [DONE]\n\n", true},
			{"失败", sse(map[string]any{"type": "response.failed"}), true},
			{"超限", sse(map[string]any{"type": "response.incomplete"}), true},
			{"JSON错误", "data: nope\n\n", true},
			{"文本冲突", delta + sse(map[string]any{"type": "response.completed", "response": textResponse("好")}), true},
		}
		for _, tt := range fixtures {
			Convey(tt.name, func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, tt.body)
				}))
				defer server.Close()
				m, err := newModel(config{APIKey: "key", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
				So(err, ShouldBeNil)
				stream, err := m.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")})
				So(err, ShouldBeNil)
				defer stream.Close()
				var chunks []*schema.Message
				for {
					chunk, e := stream.Recv()
					if e == io.EOF {
						break
					}
					if e != nil {
						err = e
						break
					}
					chunks = append(chunks, chunk)
				}
				if tt.failed {
					So(err, ShouldNotBeNil)
					return
				}
				So(err, ShouldBeNil)
				msg, err := schema.ConcatMessages(chunks)
				So(err, ShouldBeNil)
				So(msg.Content, ShouldEqual, "你好")
				So(msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens, ShouldEqual, 1024)
				So(msg.Extra[modelmeta.ProtocolKey], ShouldNotBeBlank)
			})
		}
	})
}

func TestChatModel_Errors(t *testing.T) {
	Convey("配置与上游异常", t, func() {
		for _, cfg := range []config{{}, {APIKey: "key"}, {Model: "test"}, {APIKey: "key", Model: "test", BaseURL: "ftp://example.test"}, {APIKey: "key", Model: "test", BaseURL: "https://example.test/responses"}, {APIKey: "key", Model: "test", BaseURL: "https://user:pass@example.test"}} {
			_, err := newModel(cfg)
			So(err, ShouldNotBeNil)
		}
		t.Setenv("OPENROUTER_API_KEY", "test-key")
		t.Setenv("OPENROUTER_BASE_URL", "")
		base, err := NewFromEnv(context.Background(), "vendor/pro", "medium")
		So(err, ShouldBeNil)
		m := base.(*chatModel)
		So(m.config.Model, ShouldEqual, "vendor/pro")
		So(m.config.Effort, ShouldEqual, "medium")
		So(m.config.BaseURL, ShouldEqual, "https://openrouter.ai/api/v1")
		So(m.StatelessResponses(), ShouldBeTrue)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = NewFromEnv(ctx, "vendor/pro", "medium")
		So(err, ShouldNotBeNil)
		for _, status := range []int{401, 429, 503} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret upstream payload", status) }))
			local, _ := newModel(config{APIKey: "key", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
			_, err := local.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldNotContainSubstring, "secret")
			server.Close()
		}
		_, _, err = m.request(nil, model.WithTools([]*schema.ToolInfo{nil}))
		So(err, ShouldNotBeNil)
		_, _, err = m.request(nil, model.WithTools([]*schema.ToolInfo{{Name: "same"}, {Name: "same"}}))
		So(err, ShouldNotBeNil)
		_, _, err = m.request([]*schema.Message{nil})
		So(err, ShouldNotBeNil)
		_, _, err = m.request([]*schema.Message{schema.ToolMessage("x", "")})
		So(err, ShouldNotBeNil)
		_, _, err = m.request(nil, model.WithModel("another"))
		So(err, ShouldNotBeNil)
		for _, r := range []response{{Status: "failed"}, {Status: "completed"}, {Status: "completed", Output: []json.RawMessage{json.RawMessage(`{"type":"unknown"}`)}}, toolResponse()} {
			_, err = m.message(r, nil)
			So(err, ShouldNotBeNil)
		}
	})
}

func TestChatModel_ConcurrentSessionRouting(t *testing.T) {
	Convey("并发请求保持会话隔离", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(map[string]any{"type": "response.completed", "response": textResponse(req["session_id"].(string))}))
		}))
		defer server.Close()
		m, _ := newModel(config{APIKey: "key", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
		var wg sync.WaitGroup
		results := make(chan error, 12)
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := fmt.Sprintf("session-%d", i)
				msg, err := m.Generate(modelmeta.WithSession(context.Background(), id), []*schema.Message{schema.UserMessage("hi")})
				if err == nil && msg.Content != id {
					err = fmt.Errorf("session leaked")
				}
				results <- err
			}(i)
		}
		wg.Wait()
		close(results)
		for err := range results {
			So(err, ShouldBeNil)
		}
	})
}
