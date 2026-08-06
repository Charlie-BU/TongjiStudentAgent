package taskplan

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateTaskPlanTasks(t *testing.T) {
	Convey("会话任务计划约束", t, func() {
		Convey("规范化后的有效任务可保存", func() {
			tasks, err := ValidateTaskPlanTasks([]TaskItem{{ID: " step-1 ", Desc: " 查询成绩 ", Status: TaskStatusInProgress}})

			So(err, ShouldBeNil)
			So(tasks, ShouldResemble, []TaskItem{{ID: "step-1", Desc: "查询成绩", Status: TaskStatusInProgress}})
		})

		Convey("重复 ID、未知状态与空任务计划应被拒绝", func() {
			_, err := ValidateTaskPlanTasks(nil)
			So(errors.Is(err, ErrInvalidTaskPlan), ShouldBeTrue)

			_, err = ValidateTaskPlanTasks([]TaskItem{{ID: "step1", Desc: "第一步", Status: TaskStatusPending}, {ID: "step1", Desc: "第二步", Status: TaskStatusPending}})
			So(errors.Is(err, ErrInvalidTaskPlan), ShouldBeTrue)

			_, err = ValidateTaskPlanTasks([]TaskItem{{ID: "step1", Desc: "第一步", Status: "unknown"}})
			So(errors.Is(err, ErrInvalidTaskPlan), ShouldBeTrue)
		})
	})
}
