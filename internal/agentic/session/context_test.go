package session

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestContextAssembler(t *testing.T) {
	Convey("会话上下文装配", t, func() {
		assembler := NewContextAssembler()
		input := TurnInput{
			DynamicReminder: schema.UserMessage("<system-reminder>当前日期</system-reminder>"),
			History: []Message{
				{Sequence: 1, Role: MessageRoleUser, Content: "我叫小济"},
				{Sequence: 2, Role: MessageRoleAssistant, Content: "你好，小济。"},
			},
			UserMessage: schema.UserMessage("我叫什么名字？"),
		}

		Convey("应保留消息顺序并使用模型原生角色", func() {
			messages, err := assembler.AssembleForTurn(context.Background(), input)

			So(err, ShouldBeNil)
			So(messages, ShouldHaveLength, 4)
			So(messages[0].Content, ShouldEqual, "<system-reminder>当前日期</system-reminder>")
			So(messages[1].Role, ShouldEqual, schema.User)
			So(messages[1].Content, ShouldEqual, "我叫小济")
			So(messages[2].Role, ShouldEqual, schema.Assistant)
			So(messages[2].Content, ShouldEqual, "你好，小济。")
			So(messages[3].Content, ShouldEqual, "我叫什么名字？")
		})

		Convey("应拒绝缺失当前请求、异常角色和倒序历史", func() {
			_, err := assembler.AssembleForTurn(context.Background(), TurnInput{DynamicReminder: input.DynamicReminder})
			So(errors.Is(err, ErrInvalidTurnInput), ShouldBeTrue)

			invalidRole := input
			invalidRole.History = []Message{{Sequence: 1, Role: "tool", Content: "不应进入 canonical 历史"}}
			_, err = assembler.AssembleForTurn(context.Background(), invalidRole)
			So(errors.Is(err, ErrInvalidTurnInput), ShouldBeTrue)

			outOfOrder := input
			outOfOrder.History = []Message{
				{Sequence: 2, Role: MessageRoleUser, Content: "第一条"},
				{Sequence: 1, Role: MessageRoleAssistant, Content: "第二条"},
			}
			_, err = assembler.AssembleForTurn(context.Background(), outOfOrder)
			So(errors.Is(err, ErrInvalidTurnInput), ShouldBeTrue)
		})

		Convey("应恢复工具调用、工具结果与模型 reasoning", func() {
			fullHistory := []Message{
				{Sequence: 1, Role: MessageRoleAssistant, ToolCalls: []schema.ToolCall{{ID: "call-001", Function: schema.FunctionCall{Name: "tongji.student.score", Arguments: `{"term":"2025-1"}`}}}, ReasoningContent: "需要先查询成绩"},
				{Sequence: 2, Role: MessageRoleTool, Content: `{"gpa":4.0}`, ToolCallID: "call-001", ToolName: "tongji.student.score"},
				{Sequence: 3, Role: MessageRoleAssistant, Content: "你的绩点是 4.0。"},
			}
			messages, err := assembler.AssembleForTurn(context.Background(), TurnInput{DynamicReminder: input.DynamicReminder, History: fullHistory, UserMessage: input.UserMessage})

			So(err, ShouldBeNil)
			So(messages[1].ToolCalls[0].Function.Arguments, ShouldEqual, `{"term":"2025-1"}`)
			So(messages[1].ReasoningContent, ShouldEqual, "需要先查询成绩")
			So(messages[2].Role, ShouldEqual, schema.Tool)
			So(messages[2].ToolCallID, ShouldEqual, "call-001")
			So(messages[2].Content, ShouldEqual, `{"gpa":4.0}`)
		})
	})
}
