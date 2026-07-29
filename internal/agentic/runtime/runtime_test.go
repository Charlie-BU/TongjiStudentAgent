package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRequiresChatModel(t *testing.T) {
	Convey("创建 Runtime", t, func() {
		Convey("未提供 ChatModel", func() {
			runtime, err := New(context.Background(), Config{})

			Convey("应拒绝配置并返回可定位错误", func() {
				So(runtime, ShouldBeNil)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat model is required")
			})
		})
	})
}

func TestRuntimeStreamPublishesToolFailureWhenToolStreamFails(t *testing.T) {
	Convey("工具结果流读取失败", t, func() {
		toolStream, toolWriter := schema.Pipe[*schema.Message](1)
		toolWriter.Send(nil, errors.New("tool stream failed"))
		toolWriter.Close()
		runtime := &Runtime{agent: &fakeAgent{events: []*adk.AgentEvent{
			adk.EventFromMessage(schema.AssistantMessage("", []schema.ToolCall{{
				ID: "call-1", Function: schema.FunctionCall{Name: "get_current_time"},
			}}), nil, schema.Assistant, ""),
			adk.EventFromMessage(nil, toolStream, schema.Tool, "get_current_time"),
		}}}
		var events []agentevent.Event

		Convey("应将已开始的工具调用标记为失败", func() {
			_, err := runtime.Stream(context.Background(), "现在几点？", func(event agentevent.Event) {
				events = append(events, event)
			})

			So(err, ShouldNotBeNil)
			So(events, ShouldHaveLength, 2)
			So(events[0].Type, ShouldEqual, agentevent.ToolCallStarted)
			So(events[1].Type, ShouldEqual, agentevent.ToolCallFailed)
			failedData, marshalErr := json.Marshal(events[1].Data)
			So(marshalErr, ShouldBeNil)
			So(string(failedData), ShouldContainSubstring, "call-1")
		})
	})
}

func TestChatRequiresInitializedRuntime(t *testing.T) {
	Convey("执行 Runtime 聊天", t, func() {
		Convey("Runtime 未初始化", func() {
			var runtime *Runtime
			response, err := runtime.Stream(context.Background(), "你好", nil)

			Convey("应返回初始化错误且不产生响应", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "agent runtime is not initialized")
			})
		})
	})
}

func TestRuntimeStreamPublishesSafeAssistantAndToolEvents(t *testing.T) {
	Convey("执行 Runtime 流式事件", t, func() {
		agent := &fakeAgent{events: []*adk.AgentEvent{
			adk.EventFromMessage(nil, schema.StreamReaderFromArray([]*schema.Message{
				schema.AssistantMessage("同学你好，", nil),
				schema.AssistantMessage("这里是答案。", nil),
			}), schema.Assistant, ""),
			adk.EventFromMessage(schema.AssistantMessage("", []schema.ToolCall{{
				ID: "call-1", Function: schema.FunctionCall{Name: "get_current_time", Arguments: `{"secret":"must-not-leak"}`},
			}}), nil, schema.Assistant, ""),
			adk.EventFromMessage(schema.ToolMessage("sensitive tool result", "call-1", schema.WithToolName("get_current_time")), nil, schema.Tool, "get_current_time"),
		}}
		runtime := &Runtime{agent: agent}
		var events []agentevent.Event

		Convey("应输出文本增量和脱敏工具生命周期", func() {
			response, err := runtime.Stream(context.Background(), "现在几点？", func(event agentevent.Event) {
				events = append(events, event)
			})

			So(err, ShouldBeNil)
			So(response, ShouldEqual, "同学你好，这里是答案。")
			So(events, ShouldHaveLength, 4)
			So(events[0].Type, ShouldEqual, agentevent.AssistantDelta)
			So(events[1].Type, ShouldEqual, agentevent.AssistantDelta)
			So(events[2].Type, ShouldEqual, agentevent.ToolCallStarted)
			So(events[3].Type, ShouldEqual, agentevent.ToolCallCompleted)
			startedData, marshalErr := json.Marshal(events[2].Data)
			completedData, completedMarshalErr := json.Marshal(events[3].Data)
			So(marshalErr, ShouldBeNil)
			So(completedMarshalErr, ShouldBeNil)
			So(string(startedData), ShouldNotContainSubstring, "must-not-leak")
			So(string(completedData), ShouldNotContainSubstring, "sensitive tool result")
		})
	})
}

type fakeAgent struct {
	events []*adk.AgentEvent
}

func (a *fakeAgent) Name(context.Context) string {
	return "fake-agent"
}

func (a *fakeAgent) Description(context.Context) string {
	return "fake agent for streaming tests"
}

func (a *fakeAgent) Run(_ context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		for _, event := range a.events {
			generator.Send(event)
		}
	}()
	return iterator
}
