package managetaskplan

import (
	"context"
	"encoding/json"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	. "github.com/smartystreets/goconvey/convey"
)

func TestManageTaskPlanTool(t *testing.T) {
	Convey("会话任务计划系统工具", t, func() {
		repository := &memoryRepository{}
		tool := NewTool(func(name string) bool { return name == ManageTaskPlanToolName }, repository)

		Convey("create 创建完整计划，重复创建不会覆盖", func() {
			var events []agentevent.Event
			ctx := agentevent.WithSink(context.Background(), func(event agentevent.Event) { events = append(events, event) })
			created, createErr := tool.InvokableRun(ctx, `{"action":"create","reason":"任务包含多个步骤","tasks":[{"id":"step1","desc":"查询成绩","status":"in_progress"}]}`)
			So(createErr, ShouldBeNil)
			So(decodeStatus(created), ShouldEqual, "updated")
			So(repository.plan.Revision, ShouldEqual, int64(1))
			So(events, ShouldHaveLength, 1)
			So(events[0].Type, ShouldEqual, agentevent.TaskPlanUpdated)
			data := events[0].Data.(agentevent.TaskPlanUpdatedData)
			So(data.Tasks, ShouldResemble, repository.plan.Tasks)

			again, againErr := tool.InvokableRun(context.Background(), `{"action":"create","reason":"再次创建","tasks":[{"id":"step2","desc":"不应覆盖","status":"pending"}]}`)
			So(againErr, ShouldBeNil)
			So(decodeStatus(again), ShouldEqual, "active_plan_exists")
			So(repository.plan.Tasks[0].ID, ShouldEqual, "step1")
		})

		Convey("update_status 仅更新已有任务状态", func() {
			repository.plan = &taskplan.TaskPlan{Revision: 3, Tasks: []taskplan.TaskItem{{ID: "step1", Desc: "查询成绩", Status: taskplan.TaskStatusInProgress}}}
			updated, updateErr := tool.InvokableRun(context.Background(), `{"action":"update_status","reason":"查询完成","tasks":[{"id":"step1","status":"done"}]}`)
			So(updateErr, ShouldBeNil)
			So(decodeStatus(updated), ShouldEqual, "updated")
			So(repository.plan.Tasks[0].Status, ShouldEqual, taskplan.TaskStatusDone)

			unknown, unknownErr := tool.InvokableRun(context.Background(), `{"action":"update_status","reason":"错误更新","tasks":[{"id":"step2","status":"done"}]}`)
			So(unknownErr, ShouldBeNil)
			So(decodeStatus(unknown), ShouldEqual, "unknown_task_id")
		})

		Convey("未知字段和未加白工具均被拒绝", func() {
			invalid, invalidErr := tool.InvokableRun(context.Background(), `{"action":"clear","reason":"完成","session_id":"other"}`)
			So(invalidErr, ShouldBeNil)
			So(decodeStatus(invalid), ShouldEqual, "invalid_arguments")

			denied, deniedErr := NewTool(func(string) bool { return false }, repository).InvokableRun(context.Background(), `{"action":"clear","reason":"完成"}`)
			So(deniedErr, ShouldBeNil)
			So(decodeStatus(denied), ShouldEqual, "tool_not_allowed")
		})
	})
}

func decodeStatus(encoded string) string {
	var value result
	_ = json.Unmarshal([]byte(encoded), &value)
	return value.Status
}

type memoryRepository struct {
	plan *taskplan.TaskPlan
}

func (r *memoryRepository) GetTaskPlan(context.Context) (*taskplan.TaskPlan, error) {
	return r.plan, nil
}

func (r *memoryRepository) SaveTaskPlan(_ context.Context, expectedRevision int64, tasks []taskplan.TaskItem) (taskplan.TaskPlan, error) {
	currentRevision := int64(0)
	if r.plan != nil {
		currentRevision = r.plan.Revision
	}
	if currentRevision != expectedRevision {
		return taskplan.TaskPlan{}, taskplan.ErrTaskPlanConflict
	}
	validatedTasks, err := taskplan.ValidateTaskPlanTasks(tasks)
	if err != nil {
		return taskplan.TaskPlan{}, err
	}
	r.plan = &taskplan.TaskPlan{SessionID: "session-001", Revision: currentRevision + 1, Tasks: validatedTasks}
	return *r.plan, nil
}

func (r *memoryRepository) ClearTaskPlan(_ context.Context, expectedRevision int64) error {
	if r.plan == nil {
		return taskplan.ErrTaskPlanNotFound
	}
	if r.plan.Revision != expectedRevision {
		return taskplan.ErrTaskPlanConflict
	}
	r.plan = nil
	return nil
}

var _ taskplan.TaskPlanRepository = (*memoryRepository)(nil)
