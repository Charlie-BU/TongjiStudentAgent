// Package event 定义 Agent 运行过程对外投影的 SSE 事件。
package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	RunStarted         = "run.started"
	AgentStatus        = "agent.status"
	AssistantReasoning = "assistant.reasoning"
	AssistantDelta     = "assistant.delta"
	ToolCallStarted    = "tool.call.started"
	ToolCallCompleted  = "tool.call.completed"
	ToolCallFailed     = "tool.call.failed"
	TaskPlanUpdated    = "task_plan.updated"
	RunCompleted       = "run.completed"
	RunFailed          = "run.failed"
)

// WithSink 将当前 Run 的事件出口传递给会话范围内的静态系统工具。
func WithSink(ctx context.Context, sink func(Event)) context.Context {
	return context.WithValue(ctx, eventSinkContextKey{}, sink)
}

// EmitFromContext 从系统工具向当前 Run 投影事件；缺少出口时安全忽略。
func EmitFromContext(ctx context.Context, eventType string, data any) bool {
	if ctx == nil {
		return false
	}
	sink, ok := ctx.Value(eventSinkContextKey{}).(func(Event))
	if !ok || sink == nil {
		return false
	}
	sink(Event{Type: eventType, Data: data})
	return true
}

// NewEmitter 创建单次 Agent Run 的事件发送器。
func NewEmitter(runID string, send func(Event)) *Emitter {
	// 兜底：生成随机 Run 标识和空发送函数
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
