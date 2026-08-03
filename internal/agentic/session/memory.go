package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// InMemoryStore 是仅供本地开发和单元测试使用的并发安全 Store 实现。
type InMemoryStore struct {
	mu       sync.Mutex
	sessions map[string]Session
	messages map[string][]Message
}

// NewInMemoryStore 创建空的内存会话存储。
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions: make(map[string]Session),
		messages: make(map[string][]Message),
	}
}

// Create 创建一个由 ownerUserID 拥有的持久会话。
func (s *InMemoryStore) Create(ctx context.Context, ownerUserID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return Session{}, ErrInvalidOwner
	}

	now := time.Now().UTC()
	session := Session{
		ID:           newID("ses"),
		OwnerUserID:  ownerUserID,
		Persistence:  PersistenceDurable,
		CreatedAt:    now,
		LastActiveAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return session, nil
}

// Get 读取属于 ownerUserID 的会话。
func (s *InMemoryStore) Get(ctx context.Context, sessionID, ownerUserID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return Session{}, ErrInvalidSessionID
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return Session{}, ErrInvalidOwner
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.OwnerUserID != ownerUserID {
		return Session{}, ErrNotFound
	}
	return session, nil
}

// Append 追加一条 canonical 消息，并对用户 client turn ID 保持幂等。
func (s *InMemoryStore) Append(ctx context.Context, sessionID, ownerUserID string, input NewMessage) (AppendResult, error) {
	if err := ctx.Err(); err != nil {
		return AppendResult{}, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return AppendResult{}, ErrInvalidSessionID
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return AppendResult{}, ErrInvalidOwner
	}
	input.Content = strings.TrimSpace(input.Content)
	input.ClientTurnID = strings.TrimSpace(input.ClientTurnID)
	if input.Content == "" || (input.Role != MessageRoleUser && input.Role != MessageRoleAssistant) {
		return AppendResult{}, ErrInvalidMessage
	}
	if input.Role == MessageRoleUser && input.ClientTurnID == "" {
		return AppendResult{}, ErrInvalidMessage
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.OwnerUserID != ownerUserID {
		return AppendResult{}, ErrNotFound
	}
	for _, message := range s.messages[sessionID] {
		if input.Role == MessageRoleUser && message.Role == MessageRoleUser && message.ClientTurnID == input.ClientTurnID {
			return AppendResult{Message: message}, nil
		}
	}

	now := time.Now().UTC()
	message := Message{
		ID:           newID("msg"),
		SessionID:    sessionID,
		Sequence:     int64(len(s.messages[sessionID]) + 1),
		Role:         input.Role,
		Content:      input.Content,
		ClientTurnID: input.ClientTurnID,
		CreatedAt:    now,
	}
	s.messages[sessionID] = append(s.messages[sessionID], message)
	session.LastActiveAt = now
	s.sessions[sessionID] = session
	return AppendResult{Message: message, Created: true}, nil
}

// ListMessages 按历史顺序返回最近的 limit 条消息。
func (s *InMemoryStore) ListMessages(ctx context.Context, sessionID, ownerUserID string, limit int) ([]Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrInvalidSessionID
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return nil, ErrInvalidOwner
	}
	if limit <= 0 {
		return []Message{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.OwnerUserID != ownerUserID {
		return nil, ErrNotFound
	}
	messages := s.messages[sessionID]
	start := len(messages) - limit
	if start < 0 {
		start = 0
	}
	return append([]Message(nil), messages[start:]...), nil
}

// newID 创建不包含用户身份的随机存储标识。
func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return prefix + "_" + hex.EncodeToString(value)
	}
	return prefix + "_" + time.Now().UTC().Format("20060102150405.000000000")
}
