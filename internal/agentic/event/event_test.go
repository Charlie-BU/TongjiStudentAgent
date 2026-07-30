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

func TestEmitterRejectsEventsAfterTerminalEvent(t *testing.T) {
	Convey("发送 Run 终态事件", t, func() {
		var events []Event
		emitter := NewEmitter("run-test", func(event Event) {
			events = append(events, event)
		})

		Convey("终态事件必须是同一次 Run 的最后一个事件", func() {
			So(emitter.Emit(RunCompleted, RunCompletedData{DurationMS: 1}), ShouldBeTrue)
			So(emitter.Emit(AssistantDelta, AssistantDeltaData{Text: "不应发送"}), ShouldBeFalse)

			So(events, ShouldHaveLength, 1)
			So(events[0].Type, ShouldEqual, RunCompleted)
			So(IsTerminal(events[0].Type), ShouldBeTrue)
		})
	})
}
