package openroutermodel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

// 使用真实 SDK 和本地 HTTP fixture 验证原始数据保留，禁止访问实际模型。
func TestSDKProtocolRoundTrip(t *testing.T) {
	Convey("SDK 迁移保留摘要变体和供应商扩展字段", t, func() {
		for _, streaming := range []bool{false, true} {
			result := reasoningResponse(`{"type":"reasoning","id":"rs-test","summary":["公开摘要"],"encrypted_content":"opaque","vendor_extension":{"signature":"keep-me"}}`)
			var received map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&received)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, sse(map[string]any{"type": "response.completed", "response": result}))
			}))
			t.Cleanup(server.Close)
			m, err := newModel(config{APIKey: "test", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
			So(err, ShouldBeNil)
			call := func(history []*schema.Message) (*schema.Message, error) {
				if !streaming {
					return m.Generate(context.Background(), history)
				}
				sr, err := m.Stream(context.Background(), history)
				if err != nil {
					return nil, err
				}
				defer sr.Close()
				var chunks []*schema.Message
				for {
					msg, err := sr.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						return nil, err
					}
					chunks = append(chunks, msg)
				}
				return schema.ConcatMessages(chunks)
			}
			first, err := call([]*schema.Message{schema.UserMessage("hi")})
			So(err, ShouldBeNil)
			So(first.ReasoningContent, ShouldEqual, "公开摘要")
			So(first.Extra[modelmeta.ProtocolKey], ShouldContainSubstring, "keep-me")
			_, err = call([]*schema.Message{first, schema.UserMessage("continue")})
			So(err, ShouldBeNil)
			inputs := received["input"].([]any)
			var expected map[string]any
			_ = json.Unmarshal(result.Output[0], &expected)
			So(inputs[0], ShouldResemble, expected)
			So(received["store"], ShouldEqual, false)
			So(received, ShouldNotContainKey, "previous_response_id")
			server.Close()
		}
	})
}

func TestSDKFailureHandling(t *testing.T) {
	Convey("错误脱敏、禁止隐式重试及保留取消语义", t, func() {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "secret payload", 503) }))
		defer server.Close()
		m, _ := newModel(config{APIKey: "test", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "OpenRouter HTTP 503")
		So(calls, ShouldEqual, 1)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = m.Generate(ctx, []*schema.Message{schema.UserMessage("hi")})
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		So(calls, ShouldEqual, 1)
	})
}

func TestSDKTypedOptions(t *testing.T) {
	Convey("Eino 参数通过 SDK 类型正确传给 Responses", t, func() {
		var request map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&request)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(map[string]any{"type": "response.completed", "response": textResponse("done")}))
		}))
		defer server.Close()
		m, _ := newModel(config{APIKey: "test", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
		msg, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}, model.WithTools([]*schema.ToolInfo{{Name: "test.lookup", Desc: "lookup"}}), model.WithTemperature(0.25), model.WithTopP(0.5), model.WithMaxTokens(64), model.WithToolChoice(schema.ToolChoiceForced))
		So(err, ShouldBeNil)
		So(msg.Content, ShouldEqual, "done")
		So(request["temperature"], ShouldEqual, 0.25)
		So(request["top_p"], ShouldEqual, 0.5)
		So(request["max_output_tokens"], ShouldEqual, 64)
		So(request["tool_choice"], ShouldEqual, "required")
		So(request["stream"], ShouldEqual, true)
		So(request["store"], ShouldEqual, false)
		tool := request["tools"].([]any)[0].(map[string]any)
		So(tool["name"], ShouldEqual, wireToolName("test.lookup"))
		So(tool["strict"], ShouldEqual, false)
		So(tool["parameters"], ShouldResemble, map[string]any{"type": "object", "properties": map[string]any{}})

		// 同一模型的后续请求未传工具时，不应继承上次的工具定义。
		request = nil
		_, err = m.Generate(context.Background(), []*schema.Message{schema.UserMessage("continue")})
		So(err, ShouldBeNil)
		So(request["tools"], ShouldBeEmpty)
	})
}

func TestGenerateRejectsPartialResponse(t *testing.T) {
	Convey("Generate 汇总时不将断流当成成功结果", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(map[string]any{"type": "response.output_text.delta", "delta": "partial"}))
		}))
		defer server.Close()
		m, _ := newModel(config{APIKey: "test", Model: "test", BaseURL: server.URL, HTTPClient: server.Client()})
		msg, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
		So(err, ShouldNotBeNil)
		So(msg, ShouldBeNil)
	})
}
