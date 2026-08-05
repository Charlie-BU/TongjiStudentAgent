package session

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPartitionTurns(t *testing.T) {
	Convey("按 run_id 划分完整对话 turn", t, func() {
		turns := PartitionTurns([]Message{
			{Sequence: 1, RunID: "run-001", Role: MessageRoleUser, Content: "查成绩"},
			{Sequence: 2, RunID: "run-001", Role: MessageRoleAssistant, ToolCalls: nil, Content: "正在查询"},
			{Sequence: 3, RunID: "run-002", Role: MessageRoleUser, Content: "再查课表"},
			{Sequence: 4, RunID: "run-002", Role: MessageRoleAssistant, Content: "好的"},
		})

		So(turns, ShouldHaveLength, 2)
		So(turns[0], ShouldHaveLength, 2)
		So(turns[1][0].Sequence, ShouldEqual, int64(3))
	})
}
