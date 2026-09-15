package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUnknownToolResultCancellation(t *testing.T) {
	Convey("未知工具处理保留请求取消语义", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := unknownToolResult(ctx, "unknown", "private-argument")
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		So(result, ShouldBeEmpty)
	})
}

// 使用真实 DeepAgent 工具循环与离线模型，验证未知工具回填后可纠正调用。
func TestUnknownToolRecovery(t *testing.T) {
	Convey("未知工具不终止整轮，也不阻断同批有效工具", t, func() {
		for _, mixed := range []bool{false, true} {
			name := "模型下一步改用已注册工具"
			if mixed {
				name = "同批包含未知与已注册工具"
			}
			Convey(name, func() {
				m := &unknownToolRecoveryModel{mixed: mixed}
				lookup := &recoveryLookupTool{}
				rt, err := New(context.Background(), Config{Name: "test", ChatModel: m, Tools: []tool.BaseTool{lookup}, MaxIterations: 4})
				So(err, ShouldBeNil)
				var recorded []*schema.Message
				var events []agentevent.Event
				answer, err := rt.StreamWithHistoryAndMessages(context.Background(), "查询开课记录", "", nil,
					func(e agentevent.Event) { events = append(events, e) },
					func(_ context.Context, msg *schema.Message) error { recorded = append(recorded, msg); return nil })
				So(err, ShouldBeNil)
				So(answer, ShouldEqual, "已查询课程详情中的开课记录")
				So(lookup.calls, ShouldEqual, 1)
				var unknownResults int
				for _, msg := range recorded {
					if msg.Role == schema.Tool && msg.ToolCallID == "unknown-call" {
						unknownResults++
						var result struct {
							Status string `json:"status"`
						}
						So(json.Unmarshal([]byte(msg.Content), &result), ShouldBeNil)
						So(result.Status, ShouldEqual, "tool_not_found")
						So(msg.Content, ShouldNotContainSubstring, "private-argument")
					}
				}
				So(unknownResults, ShouldEqual, 1)
				failed, completed := 0, 0
				for _, e := range events {
					So(e.Type, ShouldNotEqual, agentevent.RunFailed)
					if e.Type == agentevent.ToolCallFailed {
						failed++
						data := e.Data.(agentevent.ToolCallFailedData)
						So(data.CallID, ShouldEqual, "unknown-call")
						So(data.Code, ShouldEqual, "tool_not_found")
					}
					if e.Type == agentevent.ToolCallCompleted {
						completed++
						So(e.Data.(agentevent.ToolCallCompletedData).CallID, ShouldEqual, "valid-call")
					}
				}
				So(failed, ShouldEqual, 1)
				So(completed, ShouldEqual, 1)
			})
		}
	})
}

type recoveryLookupTool struct{ calls int }

func (*recoveryLookupTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "tongji.course.course-detail", Desc: "课程详情"}, nil
}
func (t *recoveryLookupTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	t.calls++
	return `{"offerings":[{"id":1}]}`, nil
}

type unknownToolRecoveryModel struct {
	calls int
	mixed bool
}

func (m *unknownToolRecoveryModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	valid := schema.ToolCall{ID: "valid-call", Type: "function", Function: schema.FunctionCall{Name: "tongji.course.course-detail", Arguments: `{}`}}
	if m.calls == 1 {
		calls := []schema.ToolCall{{ID: "unknown-call", Type: "function", Function: schema.FunctionCall{Name: "tongji.course.offerings", Arguments: `{"query":"private-argument"}`}}}
		if m.mixed {
			calls = append(calls, valid)
		}
		return schema.AssistantMessage("", calls), nil
	}
	var unknown, known bool
	for _, msg := range messages {
		if msg.Role != schema.Tool {
			continue
		}
		unknown = unknown || (msg.ToolCallID == "unknown-call" && strings.Contains(msg.Content, `"status":"tool_not_found"`))
		known = known || (msg.ToolCallID == "valid-call" && strings.Contains(msg.Content, `"offerings"`))
	}
	if !unknown {
		return nil, fmt.Errorf("missing unknown-tool result")
	}
	if !known {
		if m.calls > 2 || m.mixed {
			return nil, fmt.Errorf("missing registered-tool result")
		}
		return schema.AssistantMessage("", []schema.ToolCall{valid}), nil
	}
	return schema.AssistantMessage("已查询课程详情中的开课记录", nil), nil
}
func (m *unknownToolRecoveryModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}
