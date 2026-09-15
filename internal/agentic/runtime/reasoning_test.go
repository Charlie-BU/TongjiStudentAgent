package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRuntimeReasoningEvents(t *testing.T) {
	Convey("推理流实时投影且持久化不重复", t, func() {
		for _, streaming := range []bool{false, true} {
			var event *adk.AgentEvent
			if streaming {
				sr := schema.StreamReaderFromArray([]*schema.Message{
					{Role: schema.Assistant, ReasoningContent: "先查"},
					{Role: schema.Assistant, ReasoningContent: "课表"},
					{Role: schema.Assistant, Content: "完成"},
				})
				event = adk.EventFromMessage(nil, sr, schema.Assistant, "")
			} else {
				event = adk.EventFromMessage(&schema.Message{Role: schema.Assistant, ReasoningContent: "先查课表", Content: "完成"}, nil, schema.Assistant, "")
			}
			rt := &Runtime{agent: &fakeAgent{events: []*adk.AgentEvent{event}}}
			var visible string
			var deltas []string
			var records []*schema.Message
			answer, err := rt.StreamWithHistoryAndMessages(context.Background(), "hi", "", nil, func(e agentevent.Event) {
				if e.Type == agentevent.AssistantReasoning {
					delta := e.Data.(agentevent.AssistantReasoningData).Delta
					visible += delta
					deltas = append(deltas, delta)
				}
			}, func(_ context.Context, m *schema.Message) error { records = append(records, m); return nil })
			So(err, ShouldBeNil)
			So(answer, ShouldEqual, "完成")
			So(visible, ShouldEqual, "先查课表")
			if streaming {
				So(deltas, ShouldResemble, []string{"先查", "课表"})
			} else {
				So(deltas, ShouldResemble, []string{"先查课表"})
			}
			So(records, ShouldHaveLength, 1)
			So(records[0].ReasoningContent, ShouldEqual, visible)
		}
	})
}

// TestReasoningDeltaVolume 防止长推理退回逐分片重复发送完整前缀。
func TestReasoningDeltaVolume(t *testing.T) {
	Convey("长推理只发送增量且完整消息仍可持久化", t, func() {
		const count = 10000
		const delta = "abcdefghij"
		chunks := make([]*schema.Message, count)
		for i := range chunks {
			chunks[i] = &schema.Message{Role: schema.Assistant, ReasoningContent: delta}
		}
		sr := schema.StreamReaderFromArray(chunks)
		defer sr.Close()
		var totalBytes, events int
		msg, err := readMessage(&adk.MessageVariant{IsStreaming: true, Role: schema.Assistant, MessageStream: sr}, func(e agentevent.Event) {
			data, marshalErr := json.Marshal(e.Data)
			So(marshalErr, ShouldBeNil)
			So(string(data), ShouldEqual, `{"delta":"abcdefghij"}`)
			totalBytes += len(e.Data.(agentevent.AssistantReasoningData).Delta)
			events++
		})
		So(err, ShouldBeNil)
		So(events, ShouldEqual, count)
		So(totalBytes, ShouldEqual, count*len(delta))
		So(msg.ReasoningContent, ShouldEqual, strings.Repeat(delta, count))
	})
}

func TestReadMessageReasoningBeforeCompletion(t *testing.T) {
	Convey("收到推理立即发事件，不等待流结束", t, func() {
		sr, sw := schema.Pipe[*schema.Message](0)
		defer sr.Close()
		emitted := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			_, err := readMessage(&adk.MessageVariant{IsStreaming: true, Role: schema.Assistant, MessageStream: sr}, func(e agentevent.Event) {
				if e.Type == agentevent.AssistantReasoning {
					close(emitted)
				}
			})
			done <- err
		}()
		sw.Send(&schema.Message{Role: schema.Assistant, ReasoningContent: "thinking"}, nil)
		<-emitted
		sw.Send(schema.AssistantMessage("done", nil), nil)
		sw.Close()
		So(<-done, ShouldBeNil)
	})
}
