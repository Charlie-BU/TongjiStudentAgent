package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
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

func TestNewBuildsDeepAgent(t *testing.T) {
	Convey("创建 DeepAgent Runtime", t, func() {
		runtime, err := New(context.Background(), Config{
			Name:          "test-agent",
			Instruction:   "系统提示词",
			ChatModel:     &fakeToolCallingModel{},
			MaxIterations: 2,
		})

		Convey("应使用预构建 DeepAgent 而非自定义 Graph", func() {
			So(err, ShouldBeNil)
			So(runtime, ShouldNotBeNil)
			So(runtime.agent, ShouldNotBeNil)
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
			_, err := runtime.StreamWithHistory(context.Background(), "现在几点？", "", nil, func(event agentevent.Event) {
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
			response, err := runtime.StreamWithHistory(context.Background(), "你好", "", nil, nil)

			Convey("应返回初始化错误且不产生响应", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "agent runtime is not initialized")
			})
		})
	})
}

func TestRuntimeStreamCreatesIsolatedSkillRunState(t *testing.T) {
	Convey("连续执行 Agent Run", t, func() {
		agent := &fakeAgent{events: []*adk.AgentEvent{
			adk.EventFromMessage(schema.AssistantMessage("完成", nil), nil, schema.Assistant, ""),
		}}
		runtime := &Runtime{agent: agent}

		Convey("每一轮都应使用独立的 Skill Run State", func() {
			_, firstErr := runtime.StreamWithHistory(context.Background(), "第一轮", "", nil, nil)
			_, secondErr := runtime.StreamWithHistory(context.Background(), "第二轮", "", nil, nil)

			So(firstErr, ShouldBeNil)
			So(secondErr, ShouldBeNil)
			So(agent.runStates, ShouldHaveLength, 2)
			So(agent.runStates[0] == agent.runStates[1], ShouldBeFalse)
		})
	})
}

func TestRuntimeStreamContinuesAfterLoadingSkill(t *testing.T) {
	Convey("模型调用 system.load_skill", t, func() {
		chatModel := &scriptedToolCallingModel{}
		runtime, err := New(context.Background(), Config{
			Name:          "test-agent",
			Instruction:   "按需调用工具。",
			ChatModel:     chatModel,
			Tools:         systemtools.Tools(),
			MaxIterations: 3,
		})

		Convey("加载 Skill 后应继续 ReAct 并生成最终回答", func() {
			response, streamErr := runtime.StreamWithHistory(context.Background(), "请生成文档", "", nil, nil)

			So(err, ShouldBeNil)
			So(streamErr, ShouldBeNil)
			So(response, ShouldEqual, "Skill 已加载，继续执行。")
			So(chatModel.calls, ShouldEqual, 2)
			So(chatModel.sawLoadSkillResult, ShouldBeTrue)
		})
	})
}

func TestRuntimeStreamPublishesAssistantAndToolEvents(t *testing.T) {
	Convey("执行 Runtime 流式事件", t, func() {
		runtime := &Runtime{agent: &fakeAgent{events: []*adk.AgentEvent{
			adk.EventFromMessage(nil, schema.StreamReaderFromArray([]*schema.Message{
				schema.AssistantMessage("同学你好，", nil),
				schema.AssistantMessage("这里是答案。", nil),
			}), schema.Assistant, ""),
			adk.EventFromMessage(schema.AssistantMessage("", []schema.ToolCall{{
				ID: "call-1", Function: schema.FunctionCall{Name: "get_current_time", Arguments: `{"secret":"must-not-leak"}`},
			}}), nil, schema.Assistant, ""),
			adk.EventFromMessage(schema.ToolMessage("sensitive tool result", "call-1", schema.WithToolName("get_current_time")), nil, schema.Tool, "get_current_time"),
		}}}
		var events []agentevent.Event

		Convey("应输出文本增量和完整工具生命周期", func() {
			response, err := runtime.StreamWithHistory(context.Background(), "现在几点？", "", nil, func(event agentevent.Event) {
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
			So(string(startedData), ShouldContainSubstring, "must-not-leak")
			So(string(completedData), ShouldContainSubstring, "sensitive tool result")
		})
	})
}

type fakeAgent struct {
	events    []*adk.AgentEvent
	runStates []*agenticskills.RunState
}

type fakeToolCallingModel struct{}

func (*fakeToolCallingModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("Generate should not be called")
}

func (*fakeToolCallingModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("Stream should not be called")
}

func (m *fakeToolCallingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type scriptedToolCallingModel struct {
	calls              int
	sawLoadSkillResult bool
}

func (m *scriptedToolCallingModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	if m.calls == 1 {
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:   "load-skill-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      toolallowlist.LoadSkillTool,
				Arguments: `{"skill_id":"doc-generator","reason":"需要依据文档工作说明完成请求"}`,
			},
		}}), nil
	}
	for _, message := range messages {
		if message.Role == schema.Tool && message.ToolName == toolallowlist.LoadSkillTool && strings.Contains(message.Content, `"status":"ok"`) {
			m.sawLoadSkillResult = true
		}
	}
	return schema.AssistantMessage("Skill 已加载，继续执行。", nil), nil
}

func (m *scriptedToolCallingModel) Stream(ctx context.Context, messages []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.Generate(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func (m *scriptedToolCallingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (a *fakeAgent) Name(context.Context) string { return "fake-agent" }

func (a *fakeAgent) Description(context.Context) string { return "fake agent for streaming tests" }

func (a *fakeAgent) Run(ctx context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if state, ok := agenticskills.RunStateFromContext(ctx); ok {
		a.runStates = append(a.runStates, state)
	}
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		for _, event := range a.events {
			generator.Send(event)
		}
	}()
	return iterator
}

var _ tool.BaseTool = (*fakeInvokableTool)(nil)

type fakeInvokableTool struct{}

func (*fakeInvokableTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "unused"}, nil
}
