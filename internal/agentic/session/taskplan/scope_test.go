package taskplan

import (
	"context"
	"errors"
	"testing"
)

import . "github.com/smartystreets/goconvey/convey"

func TestTaskPlanScopeAndRepository(t *testing.T) {
	Convey("任务计划访问范围", t, func() {
		durable := &fakeDurableTaskPlanStore{}
		ephemeral := &fakeEphemeralTaskPlanStore{}
		repository, err := NewTaskPlanRepository(durable, ephemeral)
		So(err, ShouldBeNil)

		Convey("无 scope 的调用必须失败", func() {
			_, getErr := repository.GetTaskPlan(context.Background())
			So(errors.Is(getErr, ErrTaskPlanScopeUnavailable), ShouldBeTrue)
		})

		Convey("认证会话只能路由到 durable store 与其 owner", func() {
			scope, scopeErr := NewTaskPlanScope(Session{ID: "ses-001", OwnerUserID: "user-001", Persistence: PersistenceDurable})
			So(scopeErr, ShouldBeNil)

			_, getErr := repository.GetTaskPlan(WithTaskPlanScope(context.Background(), scope))
			So(getErr, ShouldBeNil)
			So(durable.sessionID, ShouldEqual, "ses-001")
			So(durable.ownerUserID, ShouldEqual, "user-001")
			So(ephemeral.called, ShouldBeFalse)
		})

		Convey("匿名会话只能路由到 ephemeral store", func() {
			scope, scopeErr := NewTaskPlanScope(Session{ID: "anon-001", Persistence: PersistenceEphemeral})
			So(scopeErr, ShouldBeNil)

			_, saveErr := repository.SaveTaskPlan(WithTaskPlanScope(context.Background(), scope), 0, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusPending}})
			So(saveErr, ShouldBeNil)
			So(ephemeral.sessionID, ShouldEqual, "anon-001")
		})

		Convey("伪造的会话类型和 owner 组合不能构造 scope", func() {
			_, scopeErr := NewTaskPlanScope(Session{ID: "anon-001", OwnerUserID: "user-001", Persistence: PersistenceEphemeral})
			So(errors.Is(scopeErr, ErrInvalidTaskPlanScope), ShouldBeTrue)
		})
	})
}

type fakeDurableTaskPlanStore struct {
	sessionID   string
	ownerUserID string
}

func (f *fakeDurableTaskPlanStore) GetTaskPlan(_ context.Context, sessionID, ownerUserID string) (*TaskPlan, error) {
	f.sessionID, f.ownerUserID = sessionID, ownerUserID
	return nil, nil
}

func (f *fakeDurableTaskPlanStore) SaveTaskPlan(_ context.Context, sessionID, ownerUserID string, _ int64, tasks []TaskItem) (TaskPlan, error) {
	f.sessionID, f.ownerUserID = sessionID, ownerUserID
	return TaskPlan{SessionID: sessionID, Revision: 1, Tasks: tasks}, nil
}

func (f *fakeDurableTaskPlanStore) ClearTaskPlan(_ context.Context, sessionID, ownerUserID string, _ int64) error {
	f.sessionID, f.ownerUserID = sessionID, ownerUserID
	return nil
}

type fakeEphemeralTaskPlanStore struct {
	sessionID string
	called    bool
}

func (f *fakeEphemeralTaskPlanStore) GetTaskPlan(_ context.Context, sessionID string) (*TaskPlan, error) {
	f.sessionID, f.called = sessionID, true
	return nil, nil
}

func (f *fakeEphemeralTaskPlanStore) SaveTaskPlan(_ context.Context, sessionID string, _ int64, tasks []TaskItem) (TaskPlan, error) {
	f.sessionID, f.called = sessionID, true
	return TaskPlan{SessionID: sessionID, Revision: 1, Tasks: tasks}, nil
}

func (f *fakeEphemeralTaskPlanStore) ClearTaskPlan(_ context.Context, sessionID string, _ int64) error {
	f.sessionID, f.called = sessionID, true
	return nil
}
