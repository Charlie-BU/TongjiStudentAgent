// Package postgres 实现认证会话及其任务计划的 PostgreSQL 存储。
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session = agenticsession.Session
type Persistence = agenticsession.Persistence
type Message = agenticsession.Message
type NewMessage = agenticsession.NewMessage
type AppendResult = agenticsession.AppendResult
type TaskPlan = taskplan.TaskPlan
type TaskItem = taskplan.TaskItem
type TaskStatus = taskplan.TaskStatus

const (
	PersistenceDurable   = agenticsession.PersistenceDurable
	MessageRoleUser      = agenticsession.MessageRoleUser
	MessageRoleAssistant = agenticsession.MessageRoleAssistant
	MessageRoleTool      = agenticsession.MessageRoleTool
	TaskStatusPending    = taskplan.TaskStatusPending
	TaskStatusInProgress = taskplan.TaskStatusInProgress
	TaskStatusDone       = taskplan.TaskStatusDone
	TaskStatusFailed     = taskplan.TaskStatusFailed
)

var (
	ErrInvalidOwner       = agenticsession.ErrInvalidOwner
	ErrInvalidSessionID   = agenticsession.ErrInvalidSessionID
	ErrInvalidMessage     = agenticsession.ErrInvalidMessage
	ErrInvalidTaskPlan    = taskplan.ErrInvalidTaskPlan
	ErrTaskPlanConflict   = taskplan.ErrTaskPlanConflict
	ErrTaskPlanNotFound   = taskplan.ErrTaskPlanNotFound
	ErrNotFound           = agenticsession.ErrNotFound
	newID                 = agenticsession.NewID
	validateMessage       = agenticsession.ValidateMessage
	validateTaskPlanTasks = taskplan.ValidateTaskPlanTasks
)

const postgresDSNEnv = "POSTGRES_DSN"

// PostgresStore 将已认证用户的会话和 canonical 消息持久化到 PostgreSQL。
type PostgresStore struct {
	pool postgresPool
}

// postgresPool 收敛 PostgresStore 实际使用的连接池能力，便于隔离数据库回归测试。
type postgresPool interface {
	Close()
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// NewPostgresStoreFromEnv 根据 POSTGRES_DSN 创建 PostgreSQL 会话存储。
func NewPostgresStoreFromEnv(ctx context.Context) (*PostgresStore, error) {
	return NewPostgresStore(ctx, os.Getenv(postgresDSNEnv))
}

// NewPostgresStore 创建 PostgreSQL 会话存储并验证连接。
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("%s is required", postgresDSNEnv)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL session pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL session pool: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close 关闭 PostgreSQL 连接池。
func (s *PostgresStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Create 创建由 ownerUserID 拥有的持久会话。
func (s *PostgresStore) Create(ctx context.Context, ownerUserID string) (Session, error) {
	return s.CreateWithName(ctx, ownerUserID, "")
}

// CreateWithName 创建由 ownerUserID 拥有且带名称的持久会话。
func (s *PostgresStore) CreateWithName(ctx context.Context, ownerUserID, name string) (Session, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return Session{}, ErrInvalidOwner
	}
	name = strings.TrimSpace(name)
	now := time.Now().UTC()
	result := Session{ID: newID("ses"), OwnerUserID: ownerUserID, Name: name, Persistence: PersistenceDurable, CreatedAt: now, LastActiveAt: now}
	_, err := s.pool.Exec(ctx, `INSERT INTO agent_sessions (id, owner_user_id, name, created_at, last_active_at) VALUES ($1, $2, $3, $4, $5)`, result.ID, result.OwnerUserID, result.Name, result.CreatedAt, result.LastActiveAt)
	if err != nil {
		return Session{}, fmt.Errorf("create durable session: %w", err)
	}
	return result, nil
}

// Get 读取属于 ownerUserID 的持久会话。
func (s *PostgresStore) Get(ctx context.Context, sessionID, ownerUserID string) (Session, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return Session{}, err
	}
	result := Session{Persistence: PersistenceDurable}
	err := s.pool.QueryRow(ctx, `SELECT id, owner_user_id, name, created_at, last_active_at FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&result.ID, &result.OwnerUserID, &result.Name, &result.CreatedAt, &result.LastActiveAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get durable session: %w", err)
	}
	return result, nil
}

// List 按最近活跃时间倒序读取 ownerUserID 的全部持久会话。
func (s *PostgresStore) List(ctx context.Context, ownerUserID string) ([]Session, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, ErrInvalidOwner
	}
	rows, err := s.pool.Query(ctx, `SELECT id, owner_user_id, name, created_at, last_active_at FROM agent_sessions WHERE owner_user_id = $1 ORDER BY last_active_at DESC, created_at DESC`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list durable sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]Session, 0)
	for rows.Next() {
		item := Session{Persistence: PersistenceDurable}
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.Name, &item.CreatedAt, &item.LastActiveAt); err != nil {
			return nil, fmt.Errorf("scan durable session: %w", err)
		}
		sessions = append(sessions, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate durable sessions: %w", err)
	}
	return sessions, nil
}

// Rename 修改属于 ownerUserID 的持久会话名称。
func (s *PostgresStore) Rename(ctx context.Context, sessionID, ownerUserID, name string) (Session, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return Session{}, err
	}
	name = strings.TrimSpace(name)
	result := Session{Persistence: PersistenceDurable}
	err := s.pool.QueryRow(ctx, `UPDATE agent_sessions SET name = $1 WHERE id = $2 AND owner_user_id = $3 RETURNING id, owner_user_id, name, created_at, last_active_at`, name, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&result.ID, &result.OwnerUserID, &result.Name, &result.CreatedAt, &result.LastActiveAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("rename durable session: %w", err)
	}
	return result, nil
}

// Append 追加 canonical 消息。
func (s *PostgresStore) Append(ctx context.Context, sessionID, ownerUserID string, input NewMessage) (AppendResult, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return AppendResult{}, err
	}
	input, err := validateMessage(input)
	if err != nil {
		return AppendResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AppendResult{}, fmt.Errorf("begin durable message append: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&sessionID); errors.Is(err, pgx.ErrNoRows) {
		return AppendResult{}, ErrNotFound
	} else if err != nil {
		return AppendResult{}, fmt.Errorf("lock durable session: %w", err)
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence), 0) + 1 FROM agent_session_messages WHERE session_id = $1`, sessionID).Scan(&sequence); err != nil {
		return AppendResult{}, fmt.Errorf("allocate durable message sequence: %w", err)
	}
	now := time.Now().UTC()
	toolCalls, err := json.Marshal(input.ToolCalls)
	if err != nil {
		return AppendResult{}, fmt.Errorf("marshal session tool calls: %w", err)
	}
	message := Message{ID: newID("msg"), SessionID: sessionID, RunID: input.RunID, Sequence: sequence, Role: input.Role, Content: input.Content, ToolCalls: input.ToolCalls, ToolCallID: input.ToolCallID, ToolName: input.ToolName, ReasoningContent: input.ReasoningContent, ResponseID: input.ResponseID, ResponseCacheExpiresAt: input.ResponseCacheExpiresAt, CreatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO agent_session_messages (id, session_id, run_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, response_id, response_cache_expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, message.ID, message.SessionID, message.RunID, message.Sequence, message.Role, message.Content, toolCalls, message.ToolCallID, message.ToolName, message.ReasoningContent, message.ResponseID, message.ResponseCacheExpiresAt, message.CreatedAt)
	if err != nil {
		return AppendResult{}, fmt.Errorf("insert durable message: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`, now, sessionID); err != nil {
		return AppendResult{}, fmt.Errorf("touch durable session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AppendResult{}, fmt.Errorf("commit durable message append: %w", err)
	}
	return AppendResult{Message: message, Created: true}, nil
}

// ListMessages 按历史顺序读取最近的 limit 条持久消息。
func (s *PostgresStore) ListMessages(ctx context.Context, sessionID, ownerUserID string, limit int) ([]Message, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, sessionID, ownerUserID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []Message{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, session_id, run_id, sequence, role, content, tool_calls, tool_call_id, tool_name, reasoning_content, response_id, response_cache_expires_at, created_at FROM agent_session_messages WHERE session_id = $1 ORDER BY sequence DESC LIMIT $2`, strings.TrimSpace(sessionID), limit)
	if err != nil {
		return nil, fmt.Errorf("list durable messages: %w", err)
	}
	defer rows.Close()
	messages := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		var toolCalls []byte
		if err := rows.Scan(&message.ID, &message.SessionID, &message.RunID, &message.Sequence, &message.Role, &message.Content, &toolCalls, &message.ToolCallID, &message.ToolName, &message.ReasoningContent, &message.ResponseID, &message.ResponseCacheExpiresAt, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan durable message: %w", err)
		}
		if err := json.Unmarshal(toolCalls, &message.ToolCalls); err != nil {
			return nil, fmt.Errorf("unmarshal durable tool calls: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate durable messages: %w", err)
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

// GetTaskPlan 读取属于 ownerUserID 的活动任务计划。没有计划时返回 nil。
func (s *PostgresStore) GetTaskPlan(ctx context.Context, sessionID, ownerUserID string) (*TaskPlan, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return nil, err
	}
	if _, err := s.Get(ctx, sessionID, ownerUserID); err != nil {
		return nil, err
	}
	var plan TaskPlan
	var tasks []byte
	err := s.pool.QueryRow(ctx, `SELECT revision, tasks, updated_at FROM agent_session_task_plans WHERE session_id = $1`, strings.TrimSpace(sessionID)).Scan(&plan.Revision, &tasks, &plan.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get durable task plan: %w", err)
	}
	if err := json.Unmarshal(tasks, &plan.Tasks); err != nil {
		return nil, fmt.Errorf("unmarshal durable task plan: %w", err)
	}
	plan.SessionID = strings.TrimSpace(sessionID)
	return &plan, nil
}

// SaveTaskPlan 以 expectedRevision 为前置条件保存完整任务计划。
func (s *PostgresStore) SaveTaskPlan(ctx context.Context, sessionID, ownerUserID string, expectedRevision int64, tasks []TaskItem) (TaskPlan, error) {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return TaskPlan{}, err
	}
	if expectedRevision < 0 {
		return TaskPlan{}, ErrInvalidTaskPlan
	}
	validatedTasks, err := validateTaskPlanTasks(tasks)
	if err != nil {
		return TaskPlan{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TaskPlan{}, fmt.Errorf("begin durable task plan save: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&sessionID); errors.Is(err, pgx.ErrNoRows) {
		return TaskPlan{}, ErrNotFound
	} else if err != nil {
		return TaskPlan{}, fmt.Errorf("lock durable task plan session: %w", err)
	}
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT revision FROM agent_session_task_plans WHERE session_id = $1 FOR UPDATE`, sessionID).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		currentRevision = 0
	} else if err != nil {
		return TaskPlan{}, fmt.Errorf("read durable task plan revision: %w", err)
	}
	if currentRevision != expectedRevision {
		return TaskPlan{}, ErrTaskPlanConflict
	}
	encodedTasks, err := json.Marshal(validatedTasks)
	if err != nil {
		return TaskPlan{}, fmt.Errorf("marshal durable task plan: %w", err)
	}
	now := time.Now().UTC()
	plan := TaskPlan{SessionID: sessionID, Revision: currentRevision + 1, Tasks: validatedTasks, UpdatedAt: now}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_session_task_plans (session_id, revision, tasks, updated_at) VALUES ($1, $2, $3, $4) ON CONFLICT (session_id) DO UPDATE SET revision = EXCLUDED.revision, tasks = EXCLUDED.tasks, updated_at = EXCLUDED.updated_at`, plan.SessionID, plan.Revision, encodedTasks, plan.UpdatedAt); err != nil {
		return TaskPlan{}, fmt.Errorf("save durable task plan: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`, now, sessionID); err != nil {
		return TaskPlan{}, fmt.Errorf("touch durable task plan session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TaskPlan{}, fmt.Errorf("commit durable task plan save: %w", err)
	}
	return plan, nil
}

// ClearTaskPlan 按版本条件清理活动任务计划。
func (s *PostgresStore) ClearTaskPlan(ctx context.Context, sessionID, ownerUserID string, expectedRevision int64) error {
	if err := validateOwnerAndSessionID(sessionID, ownerUserID); err != nil {
		return err
	}
	if expectedRevision <= 0 {
		return ErrInvalidTaskPlan
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin durable task plan clear: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&sessionID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock durable task plan session: %w", err)
	}
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT revision FROM agent_session_task_plans WHERE session_id = $1 FOR UPDATE`, sessionID).Scan(&currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTaskPlanNotFound
	}
	if err != nil {
		return fmt.Errorf("read durable task plan revision: %w", err)
	}
	if currentRevision != expectedRevision {
		return ErrTaskPlanConflict
	}
	if _, err := tx.Exec(ctx, `DELETE FROM agent_session_task_plans WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("clear durable task plan: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET last_active_at = $1 WHERE id = $2`, time.Now().UTC(), sessionID); err != nil {
		return fmt.Errorf("touch durable task plan session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit durable task plan clear: %w", err)
	}
	return nil
}

// EnsurePostgresSchema 创建会话存储所需的最小 PostgreSQL 表与约束。
func EnsurePostgresSchema(ctx context.Context, store *PostgresStore) error {
	if store == nil || store.pool == nil {
		return errors.New("PostgreSQL session store is required")
	}
	for _, statement := range postgresSchemaStatements {
		if _, err := store.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("ensure PostgreSQL session schema: %w", err)
		}
	}
	return nil
}

func validateOwnerAndSessionID(sessionID, ownerUserID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrInvalidSessionID
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return ErrInvalidOwner
	}
	return nil
}

var postgresSchemaStatements = []string{
	`
CREATE TABLE IF NOT EXISTS agent_sessions (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL
);
`,
	`
ALTER TABLE agent_sessions ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';
`,
	`
CREATE INDEX IF NOT EXISTS agent_sessions_owner_last_active_index
    ON agent_sessions (owner_user_id, last_active_at DESC, created_at DESC);
`,
	`
CREATE TABLE IF NOT EXISTS agent_session_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    run_id TEXT NOT NULL DEFAULT '',
    sequence BIGINT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content TEXT NOT NULL,
    tool_calls JSONB NOT NULL DEFAULT '[]',
    tool_call_id TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    reasoning_content TEXT NOT NULL DEFAULT '',
	response_id TEXT NOT NULL DEFAULT '',
	response_cache_expires_at BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (session_id, sequence)
);
`,
	`
CREATE TABLE IF NOT EXISTS agent_session_task_plans (
    session_id TEXT PRIMARY KEY REFERENCES agent_sessions(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL,
    tasks JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
`,
	`
ALTER TABLE agent_session_messages DROP CONSTRAINT IF EXISTS agent_session_messages_role_check;
`,
	`
ALTER TABLE agent_session_messages ADD CONSTRAINT agent_session_messages_role_check CHECK (role IN ('user', 'assistant', 'tool'));
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS tool_calls JSONB NOT NULL DEFAULT '[]';
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS tool_call_id TEXT NOT NULL DEFAULT '';
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS tool_name TEXT NOT NULL DEFAULT '';
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS reasoning_content TEXT NOT NULL DEFAULT '';
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS response_id TEXT NOT NULL DEFAULT '';
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS response_cache_expires_at BIGINT NOT NULL DEFAULT 0;
`,
	`
ALTER TABLE agent_session_messages ADD COLUMN IF NOT EXISTS run_id TEXT NOT NULL DEFAULT '';
`,
	`
CREATE INDEX IF NOT EXISTS agent_session_messages_session_sequence_index
    ON agent_session_messages (session_id, sequence DESC);
`,
	`
CREATE INDEX IF NOT EXISTS agent_session_messages_session_run_index
    ON agent_session_messages (session_id, run_id, sequence);
`,
}
