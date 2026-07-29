// Package event 定义 Agent 运行过程对外可投影的安全事件。
package event

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	RunStarted        = "run.started"
	AgentStatus       = "agent.status"
	AssistantDelta    = "assistant.delta"
	ToolCallStarted   = "tool.call.started"
	ToolCallCompleted = "tool.call.completed"
	ToolCallFailed    = "tool.call.failed"
	RunCompleted      = "run.completed"
	RunFailed         = "run.failed"
)

// Event 是可安全发送给调用方的单个 Agent 运行事件。
// Data 只允许携带经调用层裁剪后的展示数据，不能放入凭据、原始工具结果或模型推理内容。
type Event struct {
	Type       string    `json:"type"`
	RunID      string    `json:"run_id"`
	Sequence   int64     `json:"seq"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       any       `json:"data,omitempty"`
}

// Emitter 为单次 Run 赋予单调递增的事件序号。
type Emitter struct {
	mu       sync.Mutex
	runID    string
	sequence int64
	send     func(Event)
}

// NewEmitter 创建单次 Agent Run 的事件发送器。
func NewEmitter(runID string, send func(Event)) *Emitter {
	if runID == "" {
		runID = NewRunID()
	}
	if send == nil {
		send = func(Event) {}
	}
	return &Emitter{runID: runID, send: send}
}

// RunID 返回当前发送器所属的 Run 标识。
func (e *Emitter) RunID() string {
	return e.runID
}

// Emit 发送一个带时间和序号的事件。
func (e *Emitter) Emit(eventType string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sequence++
	e.send(Event{
		Type:       eventType,
		RunID:      e.runID,
		Sequence:   e.sequence,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	})
}

// NewRunID 生成不包含用户身份或凭据的随机 Run 标识。
func NewRunID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return "run_" + hex.EncodeToString(value)
	}
	return "run_" + time.Now().UTC().Format("20060102150405.000000000")
}
