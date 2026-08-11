package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRedisEphemeralStore(t *testing.T) {
	miniRedis := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	store, err := NewRedisEphemeralStore(context.Background(), client, time.Hour, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	Convey("匿名 Redis 会话", t, func() {
		session, err := store.Create(context.Background())

		Convey("创建后可读取，并且不带用户归属", func() {
			So(err, ShouldBeNil)
			So(session.ID, ShouldStartWith, "anon_")
			So(session.OwnerUserID, ShouldBeBlank)
			So(session.Persistence, ShouldEqual, PersistenceEphemeral)

			loaded, loadErr := store.Get(context.Background(), session.ID)
			So(loadErr, ShouldBeNil)
			So(loaded, ShouldResemble, session)
		})

		Convey("历史超过上限后保留最近消息", func() {
			first, appendErr := store.Append(context.Background(), session.ID, NewMessage{RunID: "run-001", Role: MessageRoleUser, Content: " 我是小济 "})
			So(appendErr, ShouldBeNil)
			So(first.Created, ShouldBeTrue)
			So(first.Message.Sequence, ShouldEqual, int64(1))

			assistant, appendErr := store.Append(context.Background(), session.ID, NewMessage{RunID: "run-001", Role: MessageRoleAssistant, Content: "你好，小济。", ResponseID: "resp-001", ResponseCacheExpiresAt: 1_785_000_000})
			So(appendErr, ShouldBeNil)
			So(assistant.Message.Sequence, ShouldEqual, int64(2))
			So(assistant.Message.ResponseID, ShouldEqual, "resp-001")
			last, appendErr := store.Append(context.Background(), session.ID, NewMessage{RunID: "run-002", Role: MessageRoleUser, Content: "我叫什么？"})
			So(appendErr, ShouldBeNil)
			So(last.Message.Sequence, ShouldEqual, int64(3))

			messages, listErr := store.ListMessages(context.Background(), session.ID, 10)
			So(listErr, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].Role, ShouldEqual, MessageRoleAssistant)
			So(messages[0].RunID, ShouldEqual, "run-001")
			So(messages[0].ResponseID, ShouldEqual, "resp-001")
			So(messages[0].ResponseCacheExpiresAt, ShouldEqual, int64(1_785_000_000))
			So(messages[1].Content, ShouldEqual, "我叫什么？")
			So(messages[1].RunID, ShouldEqual, "run-002")
		})

		Convey("工具调用历史可按 JSON tag 完整恢复", func() {
			assistant, appendErr := store.Append(context.Background(), session.ID, NewMessage{
				RunID: "run-001",
				Role:  MessageRoleAssistant,
				ToolCalls: []schema.ToolCall{{
					ID:       "call-001",
					Function: schema.FunctionCall{Name: "tongji.user_info", Arguments: `{}`},
				}},
			})
			So(appendErr, ShouldBeNil)
			So(assistant.Message.SessionID, ShouldEqual, session.ID)
			So(assistant.Message.ToolCalls, ShouldHaveLength, 1)
			So(assistant.Message.ToolCalls[0].ID, ShouldEqual, "call-001")

			_, appendErr = store.Append(context.Background(), session.ID, NewMessage{
				RunID: "run-001", Role: MessageRoleTool, Content: `{"status":"ok"}`,
				ToolCallID: "call-001", ToolName: "tongji.user_info",
			})
			So(appendErr, ShouldBeNil)

			messages, listErr := store.ListMessages(context.Background(), session.ID, 10)
			So(listErr, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].SessionID, ShouldEqual, session.ID)
			So(messages[0].ToolCalls, ShouldHaveLength, 1)
			So(messages[0].ToolCalls[0].ID, ShouldEqual, "call-001")
			So(messages[1].SessionID, ShouldEqual, session.ID)
			So(messages[1].ToolCallID, ShouldEqual, "call-001")
			So(messages[1].ToolName, ShouldEqual, "tongji.user_info")
		})

		Convey("任务计划按版本保存、读取和清理", func() {
			plan, saveErr := store.SaveTaskPlan(context.Background(), session.ID, 0, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusInProgress}})
			So(saveErr, ShouldBeNil)
			So(plan.Revision, ShouldEqual, int64(1))
			So(plan.SessionID, ShouldEqual, session.ID)

			loaded, loadErr := store.GetTaskPlan(context.Background(), session.ID)
			So(loadErr, ShouldBeNil)
			So(loaded, ShouldNotBeNil)
			So(loaded.Tasks, ShouldResemble, plan.Tasks)

			_, saveErr = store.SaveTaskPlan(context.Background(), session.ID, 0, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusDone}})
			So(errors.Is(saveErr, ErrTaskPlanConflict), ShouldBeTrue)

			clearErr := store.ClearTaskPlan(context.Background(), session.ID, plan.Revision)
			So(clearErr, ShouldBeNil)
			loaded, loadErr = store.GetTaskPlan(context.Background(), session.ID)
			So(loadErr, ShouldBeNil)
			So(loaded, ShouldBeNil)
		})

		Convey("追加消息会续期任务计划", func() {
			_, saveErr := store.SaveTaskPlan(context.Background(), session.ID, 0, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusPending}})
			So(saveErr, ShouldBeNil)

			miniRedis.FastForward(59 * time.Minute)
			_, appendErr := store.Append(context.Background(), session.ID, NewMessage{RunID: "run-001", Role: MessageRoleUser, Content: "继续查询"})
			So(appendErr, ShouldBeNil)
			miniRedis.FastForward(2 * time.Minute)

			plan, loadErr := store.GetTaskPlan(context.Background(), session.ID)
			So(loadErr, ShouldBeNil)
			So(plan, ShouldNotBeNil)
		})

		Convey("TTL 到期后会话不可读取", func() {
			miniRedis.FastForward(time.Hour)

			_, loadErr := store.Get(context.Background(), session.ID)
			So(errors.Is(loadErr, ErrNotFound), ShouldBeTrue)
		})

		Convey("同一会话同一时间只允许一个执行锁", func() {
			release, lockErr := store.AcquireTurn(context.Background(), session.ID)
			So(lockErr, ShouldBeNil)

			_, lockErr = store.AcquireTurn(context.Background(), session.ID)
			So(errors.Is(lockErr, ErrTurnInProgress), ShouldBeTrue)

			release()
			release, lockErr = store.AcquireTurn(context.Background(), session.ID)
			So(lockErr, ShouldBeNil)
			release()
		})
	})
}

func TestRedisEphemeralStoreValidation(t *testing.T) {
	Convey("匿名 Redis 会话配置", t, func() {
		Convey("无效 TTL 或消息上限应被拒绝", func() {
			_, err := NewRedisEphemeralStore(context.Background(), redis.NewClient(&redis.Options{}), 0, 1)
			So(errors.Is(err, ErrInvalidTTL), ShouldBeTrue)

			_, err = NewRedisEphemeralStore(context.Background(), redis.NewClient(&redis.Options{}), time.Minute, 0)
			So(errors.Is(err, ErrInvalidMessageLimit), ShouldBeTrue)
		})
	})
}
