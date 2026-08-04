package session

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPostgresStoreValidation(t *testing.T) {
	Convey("认证 PostgreSQL 会话输入", t, func() {
		Convey("会话归属与消息约束应在访问数据库前校验", func() {
			So(errors.Is(validateOwnerAndSessionID("", "user-001"), ErrInvalidSessionID), ShouldBeTrue)
			So(errors.Is(validateOwnerAndSessionID("ses-001", ""), ErrInvalidOwner), ShouldBeTrue)

			_, err := validateMessage(NewMessage{Role: MessageRoleUser, Content: " "})
			So(errors.Is(err, ErrInvalidMessage), ShouldBeTrue)
			message, err := validateMessage(NewMessage{Role: MessageRoleAssistant, Content: " 你好 "})
			So(err, ShouldBeNil)
			So(message.Content, ShouldEqual, "你好")
		})

		Convey("建表 DDL 应包含消息顺序约束", func() {
			schema := strings.Join(postgresSchemaStatements, "\n")
			So(strings.Contains(schema, "UNIQUE (session_id, sequence)"), ShouldBeTrue)
		})
	})
}

func TestPostgresStoreSchemaAndToolMessagePersistence(t *testing.T) {
	Convey("PostgreSQL 会话 schema 与工具消息", t, func() {
		pool, err := pgxmock.NewPool()
		So(err, ShouldBeNil)
		defer pool.Close()
		store := &PostgresStore{pool: pool}

		Convey("升级 schema 并读写 assistant/tool 消息", func() {
			for _, statement := range postgresSchemaStatements {
				pool.ExpectExec(regexp.QuoteMeta(statement)).WillReturnResult(pgxmock.NewResult("ALTER TABLE", 0))
			}
			So(EnsurePostgresSchema(context.Background(), store), ShouldBeNil)

			toolCalls := []schema.ToolCall{{ID: "call-001", Function: schema.FunctionCall{Name: "tongji.student.score", Arguments: `{"term":"2025-1"}`}}}
			pool.ExpectBeginTx(pgx.TxOptions{})
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM agent_sessions WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`)).
				WithArgs("ses-001", "user-001").
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("ses-001"))
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(sequence), 0) + 1 FROM agent_session_messages WHERE session_id = $1`)).
				WithArgs("ses-001").
				WillReturnRows(pgxmock.NewRows([]string{"sequence"}).AddRow(int64(1)))
			pool.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_session_messages (id, session_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`)).
				WithArgs(pgxmock.AnyArg(), "ses-001", int64(1), MessageRoleAssistant, "", pgxmock.AnyArg(), "", "", "需要查询成绩", pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			pool.ExpectExec(regexp.QuoteMeta(`UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`)).
				WithArgs(pgxmock.AnyArg(), "ses-001").
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectCommit()

			appended, appendErr := store.Append(context.Background(), "ses-001", "user-001", NewMessage{Role: MessageRoleAssistant, ToolCalls: toolCalls, ReasoningContent: "需要查询成绩"})
			So(appendErr, ShouldBeNil)
			So(appended.Message.ToolCalls, ShouldResemble, toolCalls)

			createdAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
			toolCallsJSON, marshalErr := json.Marshal(toolCalls)
			So(marshalErr, ShouldBeNil)
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id, owner_user_id, created_at, last_active_at FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`)).
				WithArgs("ses-001", "user-001").
				WillReturnRows(pgxmock.NewRows([]string{"id", "owner_user_id", "created_at", "last_active_at"}).AddRow("ses-001", "user-001", createdAt, createdAt))
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id, session_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, created_at FROM agent_session_messages WHERE session_id = $1 ORDER BY sequence DESC LIMIT $2`)).
				WithArgs("ses-001", 20).
				WillReturnRows(pgxmock.NewRows([]string{"id", "session_id", "sequence", "role", "content", "tool_calls", "tool_call_id", "tool_name", "reasoning_content", "created_at"}).
					AddRow("msg-002", "ses-001", int64(2), MessageRoleTool, `{"scores":[{"course":"高等数学"}]}`, []byte(`[]`), "call-001", "tongji.student.score", "", createdAt).
					AddRow("msg-001", "ses-001", int64(1), MessageRoleAssistant, "", toolCallsJSON, "", "", "需要查询成绩", createdAt))

			messages, listErr := store.ListMessages(context.Background(), "ses-001", "user-001", 20)
			So(listErr, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].ToolCalls, ShouldResemble, toolCalls)
			So(messages[1].Role, ShouldEqual, MessageRoleTool)
			So(messages[1].ToolCallID, ShouldEqual, "call-001")
			So(messages[1].Content, ShouldContainSubstring, "高等数学")
			So(pool.ExpectationsWereMet(), ShouldBeNil)
		})
	})
}
