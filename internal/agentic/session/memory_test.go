package session

import (
	"context"
	"errors"
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInMemoryStore(t *testing.T) {
	Convey("内存会话存储", t, func() {
		store := NewInMemoryStore()
		ctx := context.Background()

		Convey("创建并读取已认证持久会话", func() {
			session, err := store.Create(ctx, " user-001 ")

			So(err, ShouldBeNil)
			So(session.ID, ShouldStartWith, "ses_")
			So(session.OwnerUserID, ShouldEqual, "user-001")
			So(session.Persistence, ShouldEqual, PersistenceDurable)
			So(session.CreatedAt.IsZero(), ShouldBeFalse)

			loaded, err := store.Get(ctx, session.ID, "user-001")
			So(err, ShouldBeNil)
			So(loaded, ShouldResemble, session)
		})

		Convey("拒绝无 owner、跨 owner 与无效会话", func() {
			_, err := store.Create(ctx, " ")
			So(errors.Is(err, ErrInvalidOwner), ShouldBeTrue)

			session, err := store.Create(ctx, "user-001")
			So(err, ShouldBeNil)
			_, err = store.Get(ctx, session.ID, "user-002")
			So(errors.Is(err, ErrNotFound), ShouldBeTrue)
			_, err = store.Get(ctx, "", "user-001")
			So(errors.Is(err, ErrInvalidSessionID), ShouldBeTrue)
		})

		Convey("追加消息、幂等处理和最近窗口", func() {
			session, err := store.Create(ctx, "user-001")
			So(err, ShouldBeNil)

			first, err := store.Append(ctx, session.ID, "user-001", NewMessage{
				Role: MessageRoleUser, Content: "  我叫小济  ", ClientTurnID: "turn-001",
			})
			So(err, ShouldBeNil)
			So(first.Created, ShouldBeTrue)
			So(first.Message.Sequence, ShouldEqual, int64(1))
			So(first.Message.Content, ShouldEqual, "我叫小济")

			replayed, err := store.Append(ctx, session.ID, "user-001", NewMessage{
				Role: MessageRoleUser, Content: "我叫小济", ClientTurnID: "turn-001",
			})
			So(err, ShouldBeNil)
			So(replayed.Created, ShouldBeFalse)
			So(replayed.Message, ShouldResemble, first.Message)

			_, err = store.Append(ctx, session.ID, "user-001", NewMessage{
				Role: MessageRoleAssistant, Content: "你好，小济。",
			})
			So(err, ShouldBeNil)
			_, err = store.Append(ctx, session.ID, "user-001", NewMessage{
				Role: MessageRoleUser, Content: "我叫什么名字？", ClientTurnID: "turn-002",
			})
			So(err, ShouldBeNil)

			recent, err := store.ListMessages(ctx, session.ID, "user-001", 2)
			So(err, ShouldBeNil)
			So(recent, ShouldHaveLength, 2)
			So(recent[0].Role, ShouldEqual, MessageRoleAssistant)
			So(recent[1].Content, ShouldEqual, "我叫什么名字？")
		})

		Convey("拒绝不应持久化的无效消息", func() {
			session, err := store.Create(ctx, "user-001")
			So(err, ShouldBeNil)

			_, err = store.Append(ctx, session.ID, "user-001", NewMessage{Role: MessageRoleUser, Content: "", ClientTurnID: "turn-001"})
			So(errors.Is(err, ErrInvalidMessage), ShouldBeTrue)
			_, err = store.Append(ctx, session.ID, "user-001", NewMessage{Role: MessageRoleUser, Content: "你好"})
			So(errors.Is(err, ErrInvalidMessage), ShouldBeTrue)
			_, err = store.Append(ctx, session.ID, "user-002", NewMessage{Role: MessageRoleUser, Content: "你好", ClientTurnID: "turn-001"})
			So(errors.Is(err, ErrNotFound), ShouldBeTrue)
		})

		Convey("并发追加不同用户消息", func() {
			session, err := store.Create(ctx, "user-001")
			So(err, ShouldBeNil)

			const messageCount = 8
			results := make(chan AppendResult, messageCount)
			errorsCh := make(chan error, messageCount)
			var waitGroup sync.WaitGroup
			for index := 0; index < messageCount; index++ {
				waitGroup.Add(1)
				go func(index int) {
					defer waitGroup.Done()
					result, err := store.Append(ctx, session.ID, "user-001", NewMessage{
						Role:         MessageRoleUser,
						Content:      "并发消息",
						ClientTurnID: "turn-concurrent-" + string(rune('a'+index)),
					})
					if err != nil {
						errorsCh <- err
						return
					}
					results <- result
				}(index)
			}
			waitGroup.Wait()
			close(results)
			close(errorsCh)

			So(errorsCh, ShouldBeEmpty)
			So(results, ShouldHaveLength, messageCount)
			messages, err := store.ListMessages(ctx, session.ID, "user-001", messageCount)
			So(err, ShouldBeNil)
			So(messages, ShouldHaveLength, messageCount)
			for index, message := range messages {
				So(message.Sequence, ShouldEqual, int64(index+1))
			}
		})
	})
}
