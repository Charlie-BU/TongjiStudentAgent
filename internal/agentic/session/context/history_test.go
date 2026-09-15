package sessioncontext

import (
	"context"
	"testing"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestStatelessHistory(t *testing.T) {
	Convey("无状态历史保持完整工具链且忽略 Ark 缓存", t, func() {
		call := Message{Sequence: 2, Role: MessageRoleAssistant, ToolCalls: []schema.ToolCall{{ID: "c", Function: schema.FunctionCall{Name: "test", Arguments: "{}"}}}, ProtocolData: "opaque"}
		result := Message{Sequence: 3, Role: MessageRoleTool, ToolCallID: "c", Content: "result"}
		answer := Message{Sequence: 4, Role: MessageRoleAssistant, Content: "answer", ResponseID: "ark-id", ResponseCacheExpiresAt: time.Now().Add(time.Hour).Unix()}
		history := []Message{{Sequence: 1, Role: MessageRoleTool, ToolCallID: "old", Content: "orphan"}, call, result, answer}
		msgs, err := NewContextAssembler().AssembleForTurn(context.Background(), TurnInput{Stateless: true, History: history, DynamicReminder: schema.UserMessage("dynamic"), UserMessage: schema.UserMessage("new")})
		So(err, ShouldBeNil)
		So(msgs, ShouldHaveLength, 5)
		So(msgs[0].Extra[modelmeta.ProtocolKey], ShouldEqual, "opaque")
		So(msgs[2].Extra, ShouldNotContainKey, "ark-response-id")
		So(msgs[3].Content, ShouldEqual, "dynamic")
		So(completeToolHistory([]Message{call, answer}), ShouldResemble, []Message{answer})
		So(completeToolHistory([]Message{answer, call}), ShouldResemble, []Message{answer})
		So(completeToolHistory([]Message{call, result, answer}), ShouldHaveLength, 3)
	})
}

func TestInterruptedToolHistory(t *testing.T) {
	Convey("中断工具历史清理保留有效对话且不修改输入", t, func() {
		prefix := []Message{
			{Sequence: 1, Role: MessageRoleUser, Content: "只查询周一的课"},
			{Sequence: 2, Role: MessageRoleAssistant, Content: "好的"},
			{Sequence: 3, Role: MessageRoleUser, Content: "查课表和教室"},
		}
		call := Message{Sequence: 4, Role: MessageRoleAssistant, ToolCalls: []schema.ToolCall{
			{ID: "a", Function: schema.FunctionCall{Name: "timetable", Arguments: "{}"}},
			{ID: "b", Function: schema.FunctionCall{Name: "classroom", Arguments: "{}"}},
		}, ProtocolData: "interrupted-protocol"}
		a := Message{Sequence: 5, Role: MessageRoleTool, ToolCallID: "a", Content: "课表"}
		b := Message{Sequence: 6, Role: MessageRoleTool, ToolCallID: "b", Content: "教室"}
		resume := Message{Sequence: 7, Role: MessageRoleUser, Content: "继续"}
		orphan := Message{Sequence: 8, Role: MessageRoleTool, ToolCallID: "missing", Content: "孤立结果"}
		for _, tt := range []struct {
			name       string
			tail, keep []Message
		}{
			{"调用尚无结果", []Message{call}, nil},
			{"并行调用只有部分结果", []Message{call, a}, nil},
			{"中断后已有新消息", []Message{call, a, resume}, []Message{resume}},
			{"完整工具链", []Message{call, a, b}, []Message{call, a, b}},
			{"完整工具链后有孤立结果", []Message{call, a, b, orphan}, []Message{call, a, b}},
		} {
			Convey(tt.name, func() {
				history := append(append([]Message(nil), prefix...), tt.tail...)
				before := append([]Message(nil), history...)
				want := append(append([]Message(nil), prefix...), tt.keep...)
				So(completeToolHistory(history), ShouldResemble, want)
				messages, err := NewContextAssembler().AssembleForTurn(context.Background(), TurnInput{
					Stateless: true, History: history,
					DynamicReminder: schema.UserMessage("dynamic"), UserMessage: schema.UserMessage("继续处理"),
				})
				So(err, ShouldBeNil)
				So(messages, ShouldHaveLength, len(want)+2)
				So(messages[0].Content, ShouldEqual, prefix[0].Content)
				So(messages[2].Content, ShouldEqual, prefix[2].Content)
				So(history, ShouldResemble, before)
			})
		}
	})
}
