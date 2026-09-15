package openroutermodel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

// reasoningResponse 构造含有公开推理和不透明数据的完成对象。
func reasoningResponse(raw string) response {
	r := textResponse("完成")
	r.Output = append([]json.RawMessage{json.RawMessage(raw)}, r.Output...)
	return r
}

func TestReasoningRequestAndCompletion(t *testing.T) {
	Convey("请求摘要并兼容完成对象", t, func() {
		m, _ := newModel(config{APIKey: "test", Model: "test"})
		req, _, err := m.request([]*schema.Message{schema.UserMessage("hi")})
		So(err, ShouldBeNil)
		raw, err := json.Marshal(req.Options)
		So(err, ShouldBeNil)
		var fields map[string]any
		So(json.Unmarshal(raw, &fields), ShouldBeNil)
		So(fields["reasoning"], ShouldResemble, map[string]any{"summary": "auto"})

		cases := []struct{ raw, want string }{
			{`{"type":"reasoning","summary":[{"type":"summary_text","text":"先查"},{"text":"课表"}],"encrypted_content":"secret"}`, "先查课表"},
			{`{"type":"reasoning","summary":["先查","课表"]}`, "先查课表"},
			{`{"type":"reasoning","summary":[],"content":[{"type":"reasoning_text","text":"公开内容"}]}`, "公开内容"},
			{`{"type":"reasoning","summary":[{"text":"摘要"}],"content":[{"type":"reasoning_text","text":"正文"}]}`, "摘要"},
			{`{"type":"reasoning","encrypted_content":"secret","summary":[]}`, ""},
		}
		for _, tt := range cases {
			var chunks []*schema.Message
			err := m.readEvents(strings.NewReader(sse(map[string]any{"type": "response.completed", "response": reasoningResponse(tt.raw)})), nil, func(msg *schema.Message) bool {
				chunks = append(chunks, msg)
				return false
			})
			So(err, ShouldBeNil)
			msg, err := schema.ConcatMessages(chunks)
			So(err, ShouldBeNil)
			So(msg.ReasoningContent, ShouldEqual, tt.want)
			So(msg.Content, ShouldEqual, "完成")
			So(msg.ReasoningContent, ShouldNotContainSubstring, "secret")
			So(msg.Extra[modelmeta.ProtocolKey], ShouldContainSubstring, `"output"`)
		}
	})
}

func TestStreamingReasoning(t *testing.T) {
	Convey("推理增量、完成后缀和空摘要", t, func() {
		m, _ := newModel(config{APIKey: "test", Model: "test"})
		for _, eventType := range []string{"response.reasoning.delta", "response.reasoning_text.delta", "response.reasoning_summary_text.delta"} {
			Convey(eventType, func() {
				for _, completion := range []string{
					`{"type":"reasoning","summary":[{"text":"先查课表"}],"encrypted_content":"secret"}`,
					`{"type":"reasoning","content":[{"type":"reasoning_text","text":"先查课表"}]}`,
					`{"type":"reasoning","encrypted_content":"secret"}`,
				} {
					body := sse(map[string]any{"type": eventType, "output_index": 0, "delta": "先查"}) + sse(map[string]any{"type": eventType, "output_index": 0, "delta": "课表"}) + sse(map[string]any{"type": "response.completed", "response": reasoningResponse(completion)})
					var chunks []*schema.Message
					err := m.readEvents(strings.NewReader(body), nil, func(msg *schema.Message) bool { chunks = append(chunks, msg); return false })
					So(err, ShouldBeNil)
					So(chunks[0].ReasoningContent, ShouldEqual, "先查")
					msg, err := schema.ConcatMessages(chunks)
					So(err, ShouldBeNil)
					So(msg.ReasoningContent, ShouldEqual, "先查课表")
					So(msg.Content, ShouldEqual, "完成")
				}
			})
		}
		Convey("完成对象补齐后缀且不混入其他表示", func() {
			body := sse(map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "先查"}) +
				sse(map[string]any{"type": "response.reasoning_text.delta", "delta": "另一种表示"}) +
				sse(map[string]any{"type": "response.completed", "response": reasoningResponse(`{"type":"reasoning","summary":["先查课表"]}`)})
			var chunks []*schema.Message
			err := m.readEvents(strings.NewReader(body), nil, func(msg *schema.Message) bool { chunks = append(chunks, msg); return false })
			So(err, ShouldBeNil)
			msg, err := schema.ConcatMessages(chunks)
			So(err, ShouldBeNil)
			So(msg.ReasoningContent, ShouldEqual, "先查课表")
		})
		Convey("只有完成摘要时仍返回推理", func() {
			var chunks []*schema.Message
			err := m.readEvents(strings.NewReader(sse(map[string]any{"type": "response.completed", "response": reasoningResponse(`{"type":"reasoning","summary":["摘要"]}`)})), nil, func(msg *schema.Message) bool { chunks = append(chunks, msg); return false })
			So(err, ShouldBeNil)
			So(chunks[0].ReasoningContent, ShouldEqual, "摘要")
		})
		Convey("不同输出项分别去重", func() {
			r := reasoningResponse(`{"type":"reasoning","summary":["第一段"]}`)
			r.Output = append(r.Output, json.RawMessage(`{"type":"reasoning","summary":["第二段"]}`))
			body := sse(map[string]any{"type": "response.reasoning_summary_text.delta", "output_index": 0, "delta": "第一段"}) + sse(map[string]any{"type": "response.reasoning_text.delta", "output_index": 2, "delta": "第二段"}) + sse(map[string]any{"type": "response.completed", "response": r})
			var chunks []*schema.Message
			err := m.readEvents(strings.NewReader(body), nil, func(msg *schema.Message) bool { chunks = append(chunks, msg); return false })
			So(err, ShouldBeNil)
			msg, err := schema.ConcatMessages(chunks)
			So(err, ShouldBeNil)
			So(msg.ReasoningContent, ShouldEqual, "第一段第二段")
		})
	})
}
