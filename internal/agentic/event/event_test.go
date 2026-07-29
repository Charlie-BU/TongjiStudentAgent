package event

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEmitterAssignsRunIDAndIncreasingSequence(t *testing.T) {
	Convey("发送 Agent 运行事件", t, func() {
		var events []Event
		emitter := NewEmitter("run-test", func(event Event) {
			events = append(events, event)
		})

		Convey("应为同一次运行分配递增序号", func() {
			emitter.Emit(RunStarted, nil)
			emitter.Emit(AgentStatus, map[string]string{"phase": "model"})

			So(events, ShouldHaveLength, 2)
			So(events[0].RunID, ShouldEqual, "run-test")
			So(events[0].Sequence, ShouldEqual, 1)
			So(events[1].Sequence, ShouldEqual, 2)
			So(events[0].OccurredAt.IsZero(), ShouldBeFalse)
		})
	})
}
