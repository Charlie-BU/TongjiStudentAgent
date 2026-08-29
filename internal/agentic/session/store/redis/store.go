// Package redis 实现匿名会话及其任务计划的 Redis 存储。
package redis

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

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/redis/go-redis/v9"
)

type Session = agenticsession.Session
type Message = agenticsession.Message
type NewMessage = agenticsession.NewMessage
type AppendResult = agenticsession.AppendResult
type TurnRelease = agenticsession.TurnRelease
type TaskPlan = taskplan.TaskPlan
type TaskItem = taskplan.TaskItem
type TaskStatus = taskplan.TaskStatus

const (
	PersistenceEphemeral = agenticsession.PersistenceEphemeral
	MessageRoleUser      = agenticsession.MessageRoleUser
	MessageRoleAssistant = agenticsession.MessageRoleAssistant
	MessageRoleTool      = agenticsession.MessageRoleTool
	TaskStatusPending    = taskplan.TaskStatusPending
	TaskStatusInProgress = taskplan.TaskStatusInProgress
	TaskStatusDone       = taskplan.TaskStatusDone
	TaskStatusFailed     = taskplan.TaskStatusFailed
)

var (
	ErrInvalidTTL          = agenticsession.ErrInvalidTTL
	ErrInvalidMessageLimit = agenticsession.ErrInvalidMessageLimit
	ErrInvalidSessionID    = agenticsession.ErrInvalidSessionID
	ErrInvalidTaskPlan     = taskplan.ErrInvalidTaskPlan
	ErrTaskPlanConflict    = taskplan.ErrTaskPlanConflict
	ErrTaskPlanNotFound    = taskplan.ErrTaskPlanNotFound
	ErrNotFound            = agenticsession.ErrNotFound
	ErrTurnInProgress      = agenticsession.ErrTurnInProgress
	newID                  = agenticsession.NewID
	validateMessage        = agenticsession.ValidateMessage
	validateTaskPlanTasks  = taskplan.ValidateTaskPlanTasks
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
	result, err := redisAppendScript.Run(ctx, s.client, []string{redisMetaKey(sessionID), redisMessagesKey(sessionID), redisTaskPlanKey(sessionID)}, string(input.Role), input.Content, string(toolCalls), input.ToolCallID, input.ToolName, input.ReasoningContent, input.ResponseID, strconv.FormatInt(input.ResponseCacheExpiresAt, 10), input.RunID, messageID, now.Format(time.RFC3339Nano), strconv.FormatInt(s.ttl.Milliseconds(), 10), strconv.Itoa(s.maxItems)).Result()
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

// ListMessagePage 按固定 sequence 快照从最新消息向更早消息分页读取，并按历史顺序返回。
func (s *RedisEphemeralStore) ListMessagePage(ctx context.Context, sessionID string, limit, offset int, snapshotSequence int64) (agenticsession.MessagePage, error) {
	if _, err := s.Get(ctx, sessionID); err != nil {
		return agenticsession.MessagePage{}, err
	}
	if limit <= 0 || offset < 0 || snapshotSequence < 0 {
		return agenticsession.MessagePage{}, fmt.Errorf("invalid message page arguments")
	}
	encoded, err := s.client.LRange(ctx, redisMessagesKey(sessionID), 0, -1).Result()
	if err != nil {
		return agenticsession.MessagePage{}, fmt.Errorf("list ephemeral message page: %w", err)
	}
	messages := make([]Message, 0, limit+1)
	matched := 0
	for _, item := range encoded {
		var message Message
		if err := json.Unmarshal([]byte(item), &message); err != nil {
			return agenticsession.MessagePage{}, fmt.Errorf("decode ephemeral page message: %w", err)
		}
		if snapshotSequence == 0 {
			snapshotSequence = message.Sequence
		}
		if message.Sequence > snapshotSequence {
			continue
		}
		if matched < offset {
			matched++
			continue
		}
		messages = append(messages, message)
		if len(messages) == limit+1 {
			break
		}
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return agenticsession.MessagePage{Messages: messages, HasMore: hasMore, SnapshotSequence: snapshotSequence}, nil
}

// GetTaskPlan 读取匿名会话的活动任务计划。没有计划时返回 nil。
func (s *RedisEphemeralStore) GetTaskPlan(ctx context.Context, sessionID string) (*TaskPlan, error) {
	if _, err := s.Get(ctx, sessionID); err != nil {
		return nil, err
	}
	encoded, err := s.client.Get(ctx, redisTaskPlanKey(sessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ephemeral task plan: %w", err)
	}
	var plan TaskPlan
	if err := json.Unmarshal([]byte(encoded), &plan); err != nil {
		return nil, fmt.Errorf("decode ephemeral task plan: %w", err)
	}
	if plan.SessionID != sessionID {
		return nil, errors.New("invalid Redis ephemeral task plan payload")
	}
	return &plan, nil
}

// SaveTaskPlan 以 expectedRevision 为前置条件保存匿名会话的完整任务计划。
func (s *RedisEphemeralStore) SaveTaskPlan(ctx context.Context, sessionID string, expectedRevision int64, tasks []TaskItem) (TaskPlan, error) {
	if strings.TrimSpace(sessionID) == "" {
		return TaskPlan{}, ErrInvalidSessionID
	}
	if expectedRevision < 0 {
		return TaskPlan{}, ErrInvalidTaskPlan
	}
	validatedTasks, err := validateTaskPlanTasks(tasks)
	if err != nil {
		return TaskPlan{}, err
	}
	plan := TaskPlan{SessionID: sessionID, Revision: expectedRevision + 1, Tasks: validatedTasks, UpdatedAt: time.Now().UTC()}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return TaskPlan{}, fmt.Errorf("marshal ephemeral task plan: %w", err)
	}
	result, err := redisSaveTaskPlanScript.Run(ctx, s.client, []string{redisMetaKey(sessionID), redisMessagesKey(sessionID), redisTaskPlanKey(sessionID)}, strconv.FormatInt(expectedRevision, 10), string(encoded), plan.UpdatedAt.Format(time.RFC3339Nano), strconv.FormatInt(s.ttl.Milliseconds(), 10)).Int64()
	if err != nil {
		return TaskPlan{}, fmt.Errorf("save ephemeral task plan: %w", err)
	}
	switch result {
	case 1:
		return plan, nil
	case 0:
		return TaskPlan{}, ErrNotFound
	case 2:
		return TaskPlan{}, ErrTaskPlanConflict
	default:
		return TaskPlan{}, errors.New("invalid Redis ephemeral task plan save result")
	}
}

// ClearTaskPlan 按版本条件清理匿名会话的活动任务计划。
func (s *RedisEphemeralStore) ClearTaskPlan(ctx context.Context, sessionID string, expectedRevision int64) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrInvalidSessionID
	}
	if expectedRevision <= 0 {
		return ErrInvalidTaskPlan
	}
	result, err := redisClearTaskPlanScript.Run(ctx, s.client, []string{redisMetaKey(sessionID), redisMessagesKey(sessionID), redisTaskPlanKey(sessionID)}, strconv.FormatInt(expectedRevision, 10), time.Now().UTC().Format(time.RFC3339Nano), strconv.FormatInt(s.ttl.Milliseconds(), 10)).Int64()
	if err != nil {
		return fmt.Errorf("clear ephemeral task plan: %w", err)
	}
	switch result {
	case 1:
		return nil
	case 0:
		return ErrNotFound
	case 2:
		return ErrTaskPlanNotFound
	case 3:
		return ErrTaskPlanConflict
	default:
		return errors.New("invalid Redis ephemeral task plan clear result")
	}
}

func redisMetaKey(sessionID string) string { return "agent:anonymous-session:" + sessionID + ":meta" }
func redisMessagesKey(sessionID string) string {
	return "agent:anonymous-session:" + sessionID + ":messages"
}
func redisTaskPlanKey(sessionID string) string {
	return "agent:anonymous-session:" + sessionID + ":task-plan"
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
local taskPlan = KEYS[3]
if redis.call('EXISTS', meta) == 0 then return {0, ''} end
local role = ARGV[1]
local content = ARGV[2]
local toolCalls = ARGV[3]
local toolCallID = ARGV[4]
local toolName = ARGV[5]
local reasoningContent = ARGV[6]
local responseID = ARGV[7]
local responseCacheExpiresAt = tonumber(ARGV[8])
local runID = ARGV[9]
local messageID = ARGV[10]
local now = ARGV[11]
local ttl = tonumber(ARGV[12])
local maxItems = tonumber(ARGV[13])
local sequence = redis.call('HINCRBY', meta, 'next_sequence', 1)
redis.call('HSET', meta, 'last_active_at', now)
local item = cjson.encode({id=messageID, session_id=string.match(meta, '([^:]+):meta$'), run_id=runID, sequence=sequence, role=role, content=content, tool_calls=cjson.decode(toolCalls), tool_call_id=toolCallID, tool_name=toolName, reasoning_content=reasoningContent, response_id=responseID, response_cache_expires_at=responseCacheExpiresAt, created_at=now})
redis.call('LPUSH', messages, item)
redis.call('LTRIM', messages, 0, maxItems - 1)
redis.call('PEXPIRE', meta, ttl)
redis.call('PEXPIRE', messages, ttl)
redis.call('PEXPIRE', taskPlan, ttl)
return {1, item}
`)

var redisSaveTaskPlanScript = redis.NewScript(`
local meta = KEYS[1]
local messages = KEYS[2]
local taskPlan = KEYS[3]
if redis.call('EXISTS', meta) == 0 then return 0 end
local expectedRevision = tonumber(ARGV[1])
local currentRevision = 0
local existing = redis.call('GET', taskPlan)
if existing then currentRevision = tonumber(cjson.decode(existing).revision) end
if currentRevision ~= expectedRevision then return 2 end
local now = ARGV[3]
local ttl = tonumber(ARGV[4])
redis.call('SET', taskPlan, ARGV[2], 'PX', ttl)
redis.call('HSET', meta, 'last_active_at', now)
redis.call('PEXPIRE', meta, ttl)
redis.call('PEXPIRE', messages, ttl)
return 1
`)

var redisClearTaskPlanScript = redis.NewScript(`
local meta = KEYS[1]
local messages = KEYS[2]
local taskPlan = KEYS[3]
if redis.call('EXISTS', meta) == 0 then return 0 end
local existing = redis.call('GET', taskPlan)
if not existing then return 2 end
if tonumber(cjson.decode(existing).revision) ~= tonumber(ARGV[1]) then return 3 end
local ttl = tonumber(ARGV[3])
redis.call('DEL', taskPlan)
redis.call('HSET', meta, 'last_active_at', ARGV[2])
redis.call('PEXPIRE', meta, ttl)
redis.call('PEXPIRE', messages, ttl)
return 1
`)
