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

// RunStartedData 表示 Run 启动时的展示信息。
type RunStartedData struct {
	Message string `json:"message"`
}

// AgentStatusData 表示 Agent 当前执行阶段的展示信息。
type AgentStatusData struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// AssistantDeltaData 表示最终回答的单个文本增量。
type AssistantDeltaData struct {
	Text string `json:"text"`
}

// ToolCallStartedData 表示模型已选择工具且等待执行的调用。
type ToolCallStartedData struct {
	CallID      string `json:"call_id"`
	Tool        string `json:"tool"`
	DisplayName string `json:"display_name"`
}

// ToolCallCompletedData 表示本轮 Agent 已接收到工具结果。
type ToolCallCompletedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
}

// ToolCallFailedData 表示工具调用执行失败的安全错误信息。
type ToolCallFailedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// RunCompletedData 表示 Run 成功结束时的汇总信息。
type RunCompletedData struct {
	DurationMS int64 `json:"duration_ms"`
}

// RunFailedData 表示 Run 失败时的稳定错误信息。
type RunFailedData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

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
	terminal bool
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

// Emit 发送一个带时间和序号的事件，并拒绝终态后的事件。
func (e *Emitter) Emit(eventType string, data any) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.terminal {
		return false
	}
	e.sequence++
	e.send(Event{
		Type:       eventType,
		RunID:      e.runID,
		Sequence:   e.sequence,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	})
	if IsTerminal(eventType) {
		e.terminal = true
	}
	return true
}

// IsTerminal 判断事件是否表示当前 Run 已经结束。
func IsTerminal(eventType string) bool {
	return eventType == RunCompleted || eventType == RunFailed
}

// NewRunID 生成不包含用户身份或凭据的随机 Run 标识。
func NewRunID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err == nil {
		return "run_" + hex.EncodeToString(value)
	}
	return "run_" + time.Now().UTC().Format("20060102150405.000000000")
}
