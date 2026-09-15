package event

import (
	"sync"
	"time"

	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
)

type eventSinkContextKey struct{}

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

// AssistantReasoningData 表示模型明确返回的推理增量，客户端按接收顺序追加。
type AssistantReasoningData struct {
	Delta string `json:"delta"`
}

// ToolCallStartedData 表示模型已选择工具且等待执行的调用。
type ToolCallStartedData struct {
	CallID      string `json:"call_id"`
	Tool        string `json:"tool"`
	DisplayName string `json:"display_name"`
	Arguments   string `json:"arguments"`
}

// ToolCallCompletedData 表示本轮 Agent 已接收到工具结果。
type ToolCallCompletedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
	Result     string `json:"result"`
}

// ToolCallFailedData 表示工具调用执行失败的安全错误信息。
type ToolCallFailedData struct {
	CallID     string `json:"call_id"`
	Tool       string `json:"tool"`
	DurationMS int64  `json:"duration_ms"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// TaskPlanUpdatedData 是当前会话任务计划的完整最新快照。
type TaskPlanUpdatedData struct {
	Action   string              `json:"action"`
	Revision int64               `json:"revision"`
	Tasks    []taskplan.TaskItem `json:"tasks"`
}

// RunCompletedData 表示 Run 成功结束时的汇总信息。
type RunCompletedData struct {
	DurationMS int64 `json:"duration_ms"`
}

// RunFailedData 表示 Run 失败时的稳定错误信息。
type RunFailedData struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Reason     string `json:"reason,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

// Event 是单个 Agent 运行事件。
// Data 按事件类型携带前端展示数据；当前协议允许 reasoning、工具参数和工具结果，
// 但不得携带 Bearer token、数据库连接串或其他服务端凭据。
type Event struct {
	Type       string    `json:"type"`
	RunID      string    `json:"run_id"`
	SessionID  string    `json:"session_id,omitempty"`
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
