package postgres

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
			So(strings.Contains(schema, "run_id TEXT NOT NULL"), ShouldBeTrue)
			So(strings.Contains(schema, "response_id TEXT NOT NULL"), ShouldBeTrue)
			So(strings.Contains(schema, "response_cache_expires_at BIGINT NOT NULL"), ShouldBeTrue)
			So(strings.Contains(schema, "CREATE TABLE IF NOT EXISTS agent_session_task_plans"), ShouldBeTrue)
			So(strings.Contains(schema, "revision BIGINT NOT NULL"), ShouldBeTrue)
			So(strings.Contains(schema, "name TEXT NOT NULL DEFAULT ''"), ShouldBeTrue)
			So(strings.Contains(schema, "agent_sessions_owner_last_active_index"), ShouldBeTrue)
		})
	})
}

func TestPostgresStoreNamedSessionPersistence(t *testing.T) {
	Convey("PostgreSQL 会话名称、列表与重命名", t, func() {
		pool, err := pgxmock.NewPool()
		So(err, ShouldBeNil)
		defer pool.Close()
		store := &PostgresStore{pool: pool}
		originalNewID := newID
		newID = func(string) string { return "ses-001" }
		defer func() { newID = originalNewID }()

		Convey("创建时保存名称，并按归属查询和更新", func() {
			pool.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_sessions (id, owner_user_id, name, created_at, last_active_at) VALUES ($1, $2, $3, $4, $5)`)).
				WithArgs("ses-001", "user-001", "成绩查询", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			created, createErr := store.CreateWithName(context.Background(), " user-001 ", " 成绩查询 ")
			So(createErr, ShouldBeNil)
			So(created.Name, ShouldEqual, "成绩查询")

			createdAt := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id, owner_user_id, name, created_at, last_active_at FROM agent_sessions WHERE owner_user_id = $1 ORDER BY last_active_at DESC, created_at DESC`)).
				WithArgs("user-001").
				WillReturnRows(pgxmock.NewRows([]string{"id", "owner_user_id", "name", "created_at", "last_active_at"}).
					AddRow("ses-002", "user-001", "课表查询", createdAt, createdAt).
					AddRow("ses-001", "user-001", "成绩查询", createdAt, createdAt))
			sessions, listErr := store.List(context.Background(), "user-001")
			So(listErr, ShouldBeNil)
			So(sessions, ShouldHaveLength, 2)
			So(sessions[0].Name, ShouldEqual, "课表查询")

			pool.ExpectQuery(regexp.QuoteMeta(`UPDATE agent_sessions SET name = $1 WHERE id = $2 AND owner_user_id = $3 RETURNING id, owner_user_id, name, created_at, last_active_at`)).
				WithArgs("新名称", "ses-001", "user-001").
				WillReturnRows(pgxmock.NewRows([]string{"id", "owner_user_id", "name", "created_at", "last_active_at"}).AddRow("ses-001", "user-001", "新名称", createdAt, createdAt))
			renamed, renameErr := store.Rename(context.Background(), "ses-001", "user-001", " 新名称 ")
			So(renameErr, ShouldBeNil)
			So(renamed.Name, ShouldEqual, "新名称")
			So(pool.ExpectationsWereMet(), ShouldBeNil)
		})

		Convey("无 owner 时拒绝列表与重命名", func() {
			_, listErr := store.List(context.Background(), " ")
			So(errors.Is(listErr, ErrInvalidOwner), ShouldBeTrue)
			_, renameErr := store.Rename(context.Background(), "ses-001", "", "新名称")
			So(errors.Is(renameErr, ErrInvalidOwner), ShouldBeTrue)
		})

		Convey("删除时按会话归属匹配，并由外键级联关联数据", func() {
			pool.ExpectExec(regexp.QuoteMeta(`DELETE FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`)).
				WithArgs("ses-001", "user-001").
				WillReturnResult(pgxmock.NewResult("DELETE", 1))

			deleteErr := store.Delete(context.Background(), " ses-001 ", " user-001 ")

			So(deleteErr, ShouldBeNil)
			So(pool.ExpectationsWereMet(), ShouldBeNil)
		})

		Convey("删除不存在或不属于当前用户的会话返回未找到", func() {
			pool.ExpectExec(regexp.QuoteMeta(`DELETE FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`)).
				WithArgs("ses-001", "user-001").
				WillReturnResult(pgxmock.NewResult("DELETE", 0))

			deleteErr := store.Delete(context.Background(), "ses-001", "user-001")

			So(errors.Is(deleteErr, ErrNotFound), ShouldBeTrue)
			So(pool.ExpectationsWereMet(), ShouldBeNil)
		})
	})
}

func TestPostgresStoreTaskPlanPersistence(t *testing.T) {
	Convey("PostgreSQL 会话任务计划", t, func() {
		pool, err := pgxmock.NewPool()
		So(err, ShouldBeNil)
		defer pool.Close()
		store := &PostgresStore{pool: pool}

		pool.ExpectBeginTx(pgx.TxOptions{})
		pool.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM agent_sessions WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`)).
			WithArgs("ses-001", "user-001").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("ses-001"))
		pool.ExpectQuery(regexp.QuoteMeta(`SELECT revision FROM agent_session_task_plans WHERE session_id = $1 FOR UPDATE`)).
			WithArgs("ses-001").
			WillReturnError(pgx.ErrNoRows)
		pool.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_session_task_plans (session_id, revision, tasks, updated_at) VALUES ($1, $2, $3, $4) ON CONFLICT (session_id) DO UPDATE SET revision = EXCLUDED.revision, tasks = EXCLUDED.tasks, updated_at = EXCLUDED.updated_at`)).
			WithArgs("ses-001", int64(1), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		pool.ExpectExec(regexp.QuoteMeta(`UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`)).
			WithArgs(pgxmock.AnyArg(), "ses-001").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		pool.ExpectCommit()

		plan, saveErr := store.SaveTaskPlan(context.Background(), "ses-001", "user-001", 0, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusPending}})
		So(saveErr, ShouldBeNil)
		So(plan.Revision, ShouldEqual, int64(1))
		So(plan.Tasks, ShouldResemble, []TaskItem{{ID: "step1", Desc: "查询成绩", Status: TaskStatusPending}})
		So(pool.ExpectationsWereMet(), ShouldBeNil)
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
			pool.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_session_messages (id, session_id, run_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, response_id, response_cache_expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`)).
				WithArgs(pgxmock.AnyArg(), "ses-001", "run-001", int64(1), MessageRoleAssistant, "", pgxmock.AnyArg(), "", "", "需要查询成绩", "resp-001", int64(1_785_000_000), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			pool.ExpectExec(regexp.QuoteMeta(`UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`)).
				WithArgs(pgxmock.AnyArg(), "ses-001").
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			pool.ExpectCommit()

			appended, appendErr := store.Append(context.Background(), "ses-001", "user-001", NewMessage{RunID: "run-001", Role: MessageRoleAssistant, ToolCalls: toolCalls, ReasoningContent: "需要查询成绩", ResponseID: "resp-001", ResponseCacheExpiresAt: 1_785_000_000})
			So(appendErr, ShouldBeNil)
			So(appended.Message.ToolCalls, ShouldResemble, toolCalls)
			So(appended.Message.RunID, ShouldEqual, "run-001")
			So(appended.Message.ResponseID, ShouldEqual, "resp-001")

			createdAt := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
			toolCallsJSON, marshalErr := json.Marshal(toolCalls)
			So(marshalErr, ShouldBeNil)
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id, owner_user_id, name, created_at, last_active_at FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`)).
				WithArgs("ses-001", "user-001").
				WillReturnRows(pgxmock.NewRows([]string{"id", "owner_user_id", "name", "created_at", "last_active_at"}).AddRow("ses-001", "user-001", "成绩查询", createdAt, createdAt))
			pool.ExpectQuery(regexp.QuoteMeta(`SELECT id, session_id, run_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, response_id, response_cache_expires_at, created_at FROM agent_session_messages WHERE session_id = $1 ORDER BY sequence DESC LIMIT $2`)).
				WithArgs("ses-001", 20).
				WillReturnRows(pgxmock.NewRows([]string{"id", "session_id", "run_id", "sequence", "role", "content", "tool_calls", "tool_call_id", "tool_name", "reasoning_content", "response_id", "response_cache_expires_at", "created_at"}).
					AddRow("msg-002", "ses-001", "run-001", int64(2), MessageRoleTool, `{"scores":[{"course":"高等数学"}]}`, []byte(`[]`), "call-001", "tongji.student.score", "", "", int64(0), createdAt).
					AddRow("msg-001", "ses-001", "run-001", int64(1), MessageRoleAssistant, "", toolCallsJSON, "", "", "需要查询成绩", "resp-001", int64(1_785_000_000), createdAt))

			messages, listErr := store.ListMessages(context.Background(), "ses-001", "user-001", 20)
			So(listErr, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].ToolCalls, ShouldResemble, toolCalls)
			So(messages[0].RunID, ShouldEqual, "run-001")
			So(messages[0].ResponseID, ShouldEqual, "resp-001")
			So(messages[1].Role, ShouldEqual, MessageRoleTool)
			So(messages[1].ToolCallID, ShouldEqual, "call-001")
			So(messages[1].Content, ShouldContainSubstring, "高等数学")
			So(pool.ExpectationsWereMet(), ShouldBeNil)
		})
	})
}
