package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisURLEnv          = "REDIS_URL"
	ephemeralTurnLockTTL = 30 * time.Second
	ephemeralTurnRenewal = 10 * time.Second
)

// RedisEphemeralStore 将匿名会话保存在带 TTL 的 Redis 键中。
type RedisEphemeralStore struct {
	client   redis.UniversalClient
	ttl      time.Duration
	maxItems int
}

// NewRedisEphemeralStoreFromEnv 根据 REDIS_URL 创建匿名 Redis 会话存储。
func NewRedisEphemeralStoreFromEnv(ctx context.Context, ttl time.Duration, maxItems int) (*RedisEphemeralStore, error) {
	address := strings.TrimSpace(os.Getenv(redisURLEnv))
	if address == "" {
		return nil, fmt.Errorf("%s is required", redisURLEnv)
	}
	options, err := redis.ParseURL(address)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", redisURLEnv, err)
	}
	client := redis.NewClient(options)
	store, err := NewRedisEphemeralStore(ctx, client, ttl, maxItems)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return store, nil
}

// NewRedisEphemeralStore 创建匿名 Redis 会话存储并验证连接。
func NewRedisEphemeralStore(ctx context.Context, client redis.UniversalClient, ttl time.Duration, maxItems int) (*RedisEphemeralStore, error) {
	if client == nil {
		return nil, errors.New("Redis session client is required")
	}
	if ttl <= 0 {
		return nil, ErrInvalidTTL
	}
	if maxItems <= 0 {
		return nil, ErrInvalidMessageLimit
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping Redis session store: %w", err)
	}
	return &RedisEphemeralStore{client: client, ttl: ttl, maxItems: maxItems}, nil
}

// Close 关闭 Redis 客户端。
func (s *RedisEphemeralStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// AcquireTurn 为任意会话获取跨实例的短期执行锁。
func (s *RedisEphemeralStore) AcquireTurn(ctx context.Context, sessionID string) (TurnRelease, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, ErrInvalidSessionID
	}
	token := newID("turn-lock")
	locked, err := s.client.SetNX(ctx, redisTurnLockKey(sessionID), token, ephemeralTurnLockTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire session turn lock: %w", err)
	}
	if !locked {
		return nil, ErrTurnInProgress
	}

	done := make(chan struct{})
	renewed := make(chan struct{})
	go s.renewTurnLock(done, renewed, sessionID, token)

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			<-renewed
			_, _ = redisReleaseTurnLockScript.Run(context.Background(), s.client, []string{redisTurnLockKey(sessionID)}, token).Result()
		})
	}, nil
}

func (s *RedisEphemeralStore) renewTurnLock(done <-chan struct{}, renewed chan<- struct{}, sessionID, token string) {
	ticker := time.NewTicker(ephemeralTurnRenewal)
	defer ticker.Stop()
	defer close(renewed)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			renewed, err := redisRenewTurnLockScript.Run(context.Background(), s.client, []string{redisTurnLockKey(sessionID)}, token, strconv.FormatInt(ephemeralTurnLockTTL.Milliseconds(), 10)).Int64()
			if err != nil || renewed == 0 {
				return
			}
		}
	}
}

// Create 创建没有用户身份归属的临时会话。
func (s *RedisEphemeralStore) Create(ctx context.Context) (Session, error) {
	now := time.Now().UTC()
	result := Session{ID: newID("anon"), Persistence: PersistenceEphemeral, CreatedAt: now, LastActiveAt: now}
	if err := s.client.HSet(ctx, redisMetaKey(result.ID), "created_at", now.Format(time.RFC3339Nano), "last_active_at", now.Format(time.RFC3339Nano), "next_sequence", "0").Err(); err != nil {
		return Session{}, fmt.Errorf("create ephemeral session: %w", err)
	}
	if err := s.client.Expire(ctx, redisMetaKey(result.ID), s.ttl).Err(); err != nil {
		return Session{}, fmt.Errorf("set ephemeral session TTL: %w", err)
	}
	return result, nil
}

// Get 读取尚未过期的匿名会话。
func (s *RedisEphemeralStore) Get(ctx context.Context, sessionID string) (Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return Session{}, ErrInvalidSessionID
	}
	values, err := s.client.HMGet(ctx, redisMetaKey(sessionID), "created_at", "last_active_at").Result()
	if err != nil {
		return Session{}, fmt.Errorf("get ephemeral session: %w", err)
	}
	if len(values) != 2 || values[0] == nil || values[1] == nil {
		return Session{}, ErrNotFound
	}
	createdAt, err := time.Parse(time.RFC3339Nano, values[0].(string))
	if err != nil {
		return Session{}, fmt.Errorf("parse ephemeral session creation time: %w", err)
	}
	lastActiveAt, err := time.Parse(time.RFC3339Nano, values[1].(string))
	if err != nil {
		return Session{}, fmt.Errorf("parse ephemeral session activity time: %w", err)
	}
	return Session{ID: sessionID, Persistence: PersistenceEphemeral, CreatedAt: createdAt, LastActiveAt: lastActiveAt}, nil
}

// Append 追加匿名 canonical 消息。
func (s *RedisEphemeralStore) Append(ctx context.Context, sessionID string, input NewMessage) (AppendResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return AppendResult{}, ErrInvalidSessionID
	}
	input, err := validateMessage(input)
	if err != nil {
		return AppendResult{}, err
	}
	now := time.Now().UTC()
	messageID := newID("msg")
	toolCalls, err := json.Marshal(input.ToolCalls)
	if err != nil {
		return AppendResult{}, fmt.Errorf("marshal session tool calls: %w", err)
	}
	result, err := redisAppendScript.Run(ctx, s.client, []string{redisMetaKey(sessionID), redisMessagesKey(sessionID)}, string(input.Role), input.Content, string(toolCalls), input.ToolCallID, input.ToolName, input.ReasoningContent, input.RunID, messageID, now.Format(time.RFC3339Nano), strconv.FormatInt(s.ttl.Milliseconds(), 10), strconv.Itoa(s.maxItems)).Result()
	if err != nil {
		return AppendResult{}, fmt.Errorf("append ephemeral message: %w", err)
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return AppendResult{}, errors.New("invalid Redis ephemeral append result")
	}
	if code, ok := values[0].(int64); !ok || code != 1 {
		return AppendResult{}, ErrNotFound
	}
	encoded, ok := values[1].(string)
	if !ok {
		return AppendResult{}, errors.New("invalid Redis ephemeral message payload")
	}
	var message Message
	if err := json.Unmarshal([]byte(encoded), &message); err != nil {
		return AppendResult{}, fmt.Errorf("decode ephemeral message: %w", err)
	}
	return AppendResult{Message: message, Created: message.ID == messageID}, nil
}

// ListMessages 按历史顺序读取最近的匿名消息。
func (s *RedisEphemeralStore) ListMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	if _, err := s.Get(ctx, sessionID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []Message{}, nil
	}
	encoded, err := s.client.LRange(ctx, redisMessagesKey(sessionID), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("list ephemeral messages: %w", err)
	}
	messages := make([]Message, 0, len(encoded))
	for index := len(encoded) - 1; index >= 0; index-- {
		var message Message
		if err := json.Unmarshal([]byte(encoded[index]), &message); err != nil {
			return nil, fmt.Errorf("decode ephemeral message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// LoadMemory 读取匿名会话的摘要及其已覆盖的最后一条消息序号。
func (s *RedisEphemeralStore) LoadMemory(ctx context.Context, sessionID string) (MemorySnapshot, error) {
	if _, err := s.Get(ctx, sessionID); err != nil {
		return MemorySnapshot{}, err
	}
	values, err := s.client.HMGet(ctx, redisMetaKey(sessionID), "summary", "summary_anchor_sequence").Result()
	if err != nil {
		return MemorySnapshot{}, fmt.Errorf("load ephemeral session memory: %w", err)
	}
	snapshot := MemorySnapshot{}
	if len(values) > 0 && values[0] != nil {
		snapshot.Summary, _ = values[0].(string)
	}
	if len(values) > 1 && values[1] != nil {
		value, parseErr := strconv.ParseInt(values[1].(string), 10, 64)
		if parseErr != nil || value < 0 {
			return MemorySnapshot{}, fmt.Errorf("parse ephemeral session memory anchor: %w", parseErr)
		}
		snapshot.AnchorSequence = value
	}
	return snapshot, nil
}

// SaveMemory 覆盖匿名会话的派生摘要，并延长其 TTL。
func (s *RedisEphemeralStore) SaveMemory(ctx context.Context, sessionID string, snapshot MemorySnapshot) error {
	if snapshot.AnchorSequence < 0 {
		return ErrInvalidTurnInput
	}
	if _, err := s.Get(ctx, sessionID); err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, redisMetaKey(sessionID), "summary", strings.TrimSpace(snapshot.Summary), "summary_anchor_sequence", strconv.FormatInt(snapshot.AnchorSequence, 10), "last_active_at", time.Now().UTC().Format(time.RFC3339Nano))
	pipe.PExpire(ctx, redisMetaKey(sessionID), s.ttl)
	pipe.PExpire(ctx, redisMessagesKey(sessionID), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save ephemeral session memory: %w", err)
	}
	return nil
}

func redisMetaKey(sessionID string) string { return "agent:anonymous-session:" + sessionID + ":meta" }
func redisMessagesKey(sessionID string) string {
	return "agent:anonymous-session:" + sessionID + ":messages"
}
func redisTurnLockKey(sessionID string) string { return "agent:session-turn:" + sessionID }

var redisRenewTurnLockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]))
`)

var redisReleaseTurnLockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('DEL', KEYS[1])
`)

var redisAppendScript = redis.NewScript(`
local meta = KEYS[1]
local messages = KEYS[2]
if redis.call('EXISTS', meta) == 0 then return {0, ''} end
local role = ARGV[1]
local content = ARGV[2]
local toolCalls = ARGV[3]
local toolCallID = ARGV[4]
local toolName = ARGV[5]
local reasoningContent = ARGV[6]
local runID = ARGV[7]
local messageID = ARGV[8]
local now = ARGV[9]
local ttl = tonumber(ARGV[10])
local maxItems = tonumber(ARGV[11])
local sequence = redis.call('HINCRBY', meta, 'next_sequence', 1)
redis.call('HSET', meta, 'last_active_at', now)
local item = cjson.encode({ID=messageID, SessionID=string.match(meta, '([^:]+):meta$'), run_id=runID, Sequence=sequence, Role=role, Content=content, ToolCalls=cjson.decode(toolCalls), ToolCallID=toolCallID, ToolName=toolName, ReasoningContent=reasoningContent, CreatedAt=now})
redis.call('LPUSH', messages, item)
redis.call('LTRIM', messages, 0, maxItems - 1)
redis.call('PEXPIRE', meta, ttl)
redis.call('PEXPIRE', messages, ttl)
return {1, item}
`)
