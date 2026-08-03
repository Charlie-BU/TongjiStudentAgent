// Package session 定义多轮对话会话的纯领域契约。
package session

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrInvalidOwner 表示会话 owner 无效。
	ErrInvalidOwner = errors.New("session owner user ID is required")
	// ErrInvalidSessionID 表示会话 ID 无效。
	ErrInvalidSessionID = errors.New("session ID is required")
	// ErrInvalidMessage 表示消息不符合会话持久化约束。
	ErrInvalidMessage = errors.New("session message is invalid")
	// ErrInvalidTurnInput 表示上下文装配输入无效。
	ErrInvalidTurnInput = errors.New("session turn input is invalid")
	// ErrNotFound 表示会话不存在。
	ErrNotFound = errors.New("session not found")
)

// Persistence 表示会话的生命周期类型。
type Persistence string

const (
	// PersistenceDurable 表示可跨进程恢复的已认证会话。
	PersistenceDurable Persistence = "durable"
	// PersistenceEphemeral 表示后续 Redis 管理的匿名临时会话。
	PersistenceEphemeral Persistence = "ephemeral"
)

// MessageRole 表示 canonical 对话消息的角色。
type MessageRole string

const (
	// MessageRoleUser 表示用户输入。
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant 表示最终助手文本。
	MessageRoleAssistant MessageRole = "assistant"
)

// Session 表示一个已认证用户拥有的持久会话。
type Session struct {
	ID           string
	OwnerUserID  string
	Persistence  Persistence
	CreatedAt    time.Time
	LastActiveAt time.Time
}

// Message 表示可作为后续模型输入的 canonical 对话消息。
type Message struct {
	ID           string
	SessionID    string
	Sequence     int64
	Role         MessageRole
	Content      string
	ClientTurnID string
	CreatedAt    time.Time
}

// NewMessage 描述待追加的 canonical 对话消息。
type NewMessage struct {
	Role         MessageRole
	Content      string
	ClientTurnID string
}

// AppendResult 表示追加消息的结果。
type AppendResult struct {
	Message Message
	Created bool
}

// Store 定义已认证持久会话的最小存储契约。
type Store interface {
	Create(ctx context.Context, ownerUserID string) (Session, error)
	Get(ctx context.Context, sessionID, ownerUserID string) (Session, error)
	Append(ctx context.Context, sessionID, ownerUserID string, message NewMessage) (AppendResult, error)
	ListMessages(ctx context.Context, sessionID, ownerUserID string, limit int) ([]Message, error)
}
