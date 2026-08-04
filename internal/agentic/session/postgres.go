package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const postgresDSNEnv = "POSTGRES_DSN"

// PostgresStore 将已认证用户的会话和 canonical 消息持久化到 PostgreSQL。
type PostgresStore struct {
	pool *pgxpool.Pool
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
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return Session{}, ErrInvalidOwner
	}
	now := time.Now().UTC()
	result := Session{ID: newID("ses"), OwnerUserID: ownerUserID, Persistence: PersistenceDurable, CreatedAt: now, LastActiveAt: now}
	_, err := s.pool.Exec(ctx, `INSERT INTO agent_sessions (id, owner_user_id, created_at, last_active_at) VALUES ($1, $2, $3, $4)`, result.ID, result.OwnerUserID, result.CreatedAt, result.LastActiveAt)
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
	err := s.pool.QueryRow(ctx, `SELECT id, owner_user_id, created_at, last_active_at FROM agent_sessions WHERE id = $1 AND owner_user_id = $2`, strings.TrimSpace(sessionID), strings.TrimSpace(ownerUserID)).Scan(&result.ID, &result.OwnerUserID, &result.CreatedAt, &result.LastActiveAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get durable session: %w", err)
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
	message := Message{ID: newID("msg"), SessionID: sessionID, Sequence: sequence, Role: input.Role, Content: input.Content, CreatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO agent_session_messages (id, session_id, sequence, role, content, created_at) VALUES ($1, $2, $3, $4, $5, $6)`, message.ID, message.SessionID, message.Sequence, message.Role, message.Content, message.CreatedAt)
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
	rows, err := s.pool.Query(ctx, `SELECT id, session_id, sequence, role, content, created_at FROM agent_session_messages WHERE session_id = $1 ORDER BY sequence DESC LIMIT $2`, strings.TrimSpace(sessionID), limit)
	if err != nil {
		return nil, fmt.Errorf("list durable messages: %w", err)
	}
	defer rows.Close()
	messages := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.SessionID, &message.Sequence, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan durable message: %w", err)
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
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL
);
`,
	`
CREATE TABLE IF NOT EXISTS agent_session_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (session_id, sequence)
);
`,
	`
CREATE INDEX IF NOT EXISTS agent_session_messages_session_sequence_index
    ON agent_session_messages (session_id, sequence DESC);
`,
}
