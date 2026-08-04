package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
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
			first, appendErr := store.Append(context.Background(), session.ID, NewMessage{Role: MessageRoleUser, Content: " 我是小济 "})
			So(appendErr, ShouldBeNil)
			So(first.Created, ShouldBeTrue)
			So(first.Message.Sequence, ShouldEqual, int64(1))

			assistant, appendErr := store.Append(context.Background(), session.ID, NewMessage{Role: MessageRoleAssistant, Content: "你好，小济。"})
			So(appendErr, ShouldBeNil)
			So(assistant.Message.Sequence, ShouldEqual, int64(2))
			last, appendErr := store.Append(context.Background(), session.ID, NewMessage{Role: MessageRoleUser, Content: "我叫什么？"})
			So(appendErr, ShouldBeNil)
			So(last.Message.Sequence, ShouldEqual, int64(3))

			messages, listErr := store.ListMessages(context.Background(), session.ID, 10)
			So(listErr, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].Role, ShouldEqual, MessageRoleAssistant)
			So(messages[1].Content, ShouldEqual, "我叫什么？")
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
