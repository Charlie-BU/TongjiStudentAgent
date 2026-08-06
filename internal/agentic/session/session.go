// Package session 定义多轮对话会话的纯领域契约。
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
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
	// ErrInvalidTTL 表示匿名会话存活时间无效。
	ErrInvalidTTL = errors.New("session TTL must be positive")
	// ErrInvalidMessageLimit 表示会话消息数量上限无效。
	ErrInvalidMessageLimit = errors.New("session message limit must be positive")
	// ErrTurnInProgress 表示同一用户消息正在生成回复。
	ErrTurnInProgress = errors.New("session turn is already in progress")
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
	// MessageRoleTool 表示工具调用结果。
	MessageRoleTool MessageRole = "tool"
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
	ID                     string            `json:"id"`
	SessionID              string            `json:"session_id"`
	RunID                  string            `json:"run_id"`
	Sequence               int64             `json:"sequence"`
	Role                   MessageRole       `json:"role"`
	Content                string            `json:"content"`
	ToolCalls              []schema.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID             string            `json:"tool_call_id,omitempty"`
	ToolName               string            `json:"tool_name,omitempty"`
	ReasoningContent       string            `json:"reasoning_content,omitempty"`
	ResponseID             string            `json:"response_id,omitempty"`
	ResponseCacheExpiresAt int64             `json:"response_cache_expires_at,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
}

// NewMessage 描述待追加的 canonical 对话消息。
type NewMessage struct {
	RunID                  string
	Role                   MessageRole
	Content                string
	ToolCalls              []schema.ToolCall
	ToolCallID             string
	ToolName               string
	ReasoningContent       string
	ResponseID             string
	ResponseCacheExpiresAt int64
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

// EphemeralStore 定义匿名临时会话的最小存储契约。
type EphemeralStore interface {
	Create(ctx context.Context) (Session, error)
	Get(ctx context.Context, sessionID string) (Session, error)
	Append(ctx context.Context, sessionID string, message NewMessage) (AppendResult, error)
	ListMessages(ctx context.Context, sessionID string, limit int) ([]Message, error)
}

// TurnRelease 在本轮会话执行完成后释放互斥锁。
type TurnRelease func()

// TurnLocker 约束同一会话在任意实例中同一时间只执行一个 turn。
type TurnLocker interface {
	AcquireTurn(ctx context.Context, sessionID string) (TurnRelease, error)
}

// ValidateMessage 规范化并校验待写入的 canonical 消息。
func ValidateMessage(input NewMessage) (NewMessage, error) {
	input.Content = strings.TrimSpace(input.Content)
	if input.Role != MessageRoleUser && input.Role != MessageRoleAssistant && input.Role != MessageRoleTool {
		return NewMessage{}, ErrInvalidMessage
	}
	if input.Content == "" && len(input.ToolCalls) == 0 && input.ReasoningContent == "" {
		return NewMessage{}, ErrInvalidMessage
	}
	if input.Role == MessageRoleTool && strings.TrimSpace(input.ToolCallID) == "" {
		return NewMessage{}, ErrInvalidMessage
	}
	return input, nil
}

// NewMessageFromSchema 将 Agent 输出转换为可持久化的会话消息。
func NewMessageFromSchema(message *schema.Message) (NewMessage, error) {
	if message == nil {
		return NewMessage{}, ErrInvalidMessage
	}
	input := NewMessage{Content: message.Content, ToolCalls: message.ToolCalls, ToolCallID: message.ToolCallID, ToolName: message.ToolName, ReasoningContent: message.ReasoningContent}
	switch message.Role {
	case schema.Assistant:
		input.Role = MessageRoleAssistant
	case schema.Tool:
		input.Role = MessageRoleTool
	default:
		return NewMessage{}, ErrInvalidMessage
	}
	return ValidateMessage(input)
}

// NewID 创建不包含用户身份的随机存储标识。
func NewID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return prefix + "_" + hex.EncodeToString(value)
	}
	return prefix + "_" + time.Now().UTC().Format("20060102150405.000000000")
}

func newID(prefix string) string { return NewID(prefix) }
