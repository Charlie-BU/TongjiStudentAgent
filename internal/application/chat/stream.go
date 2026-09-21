package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/cloudwego/eino/schema"
)

var httpStatusCodePattern = regexp.MustCompile(`(?i)\b(?:http|status(?:\s+code)?|code)\s*[:=]?\s*([1-5][0-9]{2})\b`)

// StreamSession 提交会话消息并以 SSE 事件返回本轮执行过程。
func StreamSession(ctx context.Context, sessionID string, query string, send func(agentevent.Event), tier string) (string, error) {
	if defaultService == nil {
		return "", fmt.Errorf("chat service is not initialized")
	}
	return defaultService.StreamSession(ctx, sessionID, query, send, tier)
}

// StreamSession 使用明确指定的模型档位，将当前用户消息、历史和最终回答写入同一会话。
// 参数：
//
//	ctx: 上下文，包含模型档位和会话信息。
//	sessionID: 会话 ID，用于唯一标识会话。
//	query: 用户输入的查询，包含问题或指令。
//	send: 用于发送 SSE 事件的回调函数。
//	tier: 模型档位，用于选择要使用的模型。
func (s *Service) StreamSession(ctx context.Context, sessionID string, query string, send func(agentevent.Event), tier string) (string, error) {
	curr_runtime, err := s.resolveModelTier(tier)
	if err != nil {
		return "", err
	}
	// 把模型信息挂到 context
	ctx = context.WithValue(ctx, modelSelectionKey{}, modelSelection{tier: tier, modelID: curr_runtime.modelID})
	// 把会话信息挂到 context
	ctx = modelmeta.WithSession(ctx, sessionID)
	runID := agentevent.NewRunID()
	// 给当前会话 sessionID 加“本轮执行锁”，保证同一个会话同一时间只能跑一个回答流程
	releaseTurn, err := s.acquireSessionTurn(ctx, sessionID)
	if err != nil {
		emitSessionFailure(runID, send, err)
		return "", err
	}
	defer releaseTurn() // 如果拿到了锁，把释放动作延迟到函数结束时执行
	// 创建 taskPlanScope
	scope, err := s.taskPlanScope(ctx, sessionID)
	if err != nil {
		emitSessionFailure(runID, send, err)
		return "", err
	}
	// 绑定 taskPlanScope 到 context
	runCtx := taskplan.WithTaskPlanScope(ctx, scope)
	activeTaskPlan, err := s.GetSessionTaskPlan(runCtx, sessionID)
	if err != nil {
		emitSessionFailure(runID, send, err)
		return "", err
	}
	// 绑定 activeTaskPlan 到 context
	runCtx = taskplan.WithActiveTaskPlan(runCtx, activeTaskPlan)

	// 为本轮选择存储、从存储中读取历史并构造用户消息追加操作
	history, appendUser, err := s.sessionTurnOperations(runCtx, sessionID, query, runID)
	if err != nil {
		emitSessionFailure(runID, send, err)
		return "", err
	}
	history = sanitizeHistoryForModel(history, curr_runtime.modelID) // 清理当前模型连续会话段之前的协议数据与响应缓存信息

	// 模型调用前回调：追加用户消息到 session
	beforeModel := func() error {
		_, err := appendUser()
		return err
	}
	// 记录回调：追加 Agent 输出消息到 session
	record := func(ctx context.Context, message *schema.Message) error {
		return s.appendAgentMessage(ctx, sessionID, runID, message)
	}

	return s.stream(runCtx, curr_runtime.runtime, runID, query, history, beforeModel, record, send)
}

// stream 执行一次模型调用，并通过 record 持久化每条 Agent 输出消息。
// 参数：上下文、本轮运行时、本轮 runID、用户 query、历史消息、模型调用前回调、记录回调、事件发送函数
// 返回：模型回答、错误信息
func (s *Service) stream(ctx context.Context, runner sessionRuntime, runID, query string, history []agenticsession.Message, beforeModel func() error, record func(context.Context, *schema.Message) error, send func(agentevent.Event)) (string, error) {
	emitter := agentevent.NewEmitter(runID, send)
	if s == nil || runner == nil {
		emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
		emitter.Emit(agentevent.RunFailed, runFailedData("agent_unavailable", "Agent 服务暂不可用", fmt.Errorf("chat service is not initialized")))
		return "", fmt.Errorf("chat service is not initialized")
	}
	startedAt := time.Now()
	emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
	emitter.Emit(agentevent.AgentStatus, agentevent.AgentStatusData{Phase: "context", Message: "正在准备回答上下文"})
	studentInfo, err := s.loadFormattedStudentInfo(ctx)
	if err != nil {
		// 学生资料仅用于补充上下文；无效凭据、非学生或上游异常均不阻塞普通对话。
		// 请求取消仍需结束本轮，不能继续调用模型。
		if ctx.Err() != nil {
			emitter.Emit(agentevent.RunFailed, runFailedData("agent_execution_failed", "Agent 执行失败", ctx.Err()))
			return "", ctx.Err()
		}
		slog.WarnContext(ctx, "Student context unavailable; continuing without student info", "run_id", runID)
		studentInfo = ""
	}
	// Agent 执行前
	if beforeModel != nil {
		if err = beforeModel(); err != nil {
			code, message := "session_write_failed", "会话消息暂时无法保存，请稍后重试"
			if errors.Is(err, agenticsession.ErrTurnInProgress) {
				code, message = "turn_in_progress", "该消息正在处理中，请勿重复提交"
			}
			emitter.Emit(agentevent.RunFailed, runFailedData(code, message, err))
			return "", err
		}
	}

	emitter.Emit(agentevent.AgentStatus, agentevent.AgentStatusData{Phase: "model", Message: "正在生成回答"})
	emitRuntimeEvent := func(event agentevent.Event) {
		emitter.Emit(event.Type, event.Data)
	}
	response, err := runner.StreamWithHistoryAndMessages(ctx, query, studentInfo, history, emitRuntimeEvent, record)
	if err != nil {
		emitter.Emit(agentevent.RunFailed, runFailedData("agent_execution_failed", "Agent 执行失败", err))
		return "", err
	}
	emitter.Emit(agentevent.RunCompleted, agentevent.RunCompletedData{DurationMS: time.Since(startedAt).Milliseconds()})
	return response, nil
}

// emitSessionFailure 发送会话失败事件。
func emitSessionFailure(runID string, send func(agentevent.Event), err error) {
	emitter := agentevent.NewEmitter(runID, send)
	emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
	code, message := "session_unavailable", "会话不存在或暂时不可用"
	if errors.Is(err, agenticsession.ErrTurnInProgress) {
		code, message = "turn_in_progress", "该会话正在处理中，请稍后重试"
	}
	emitter.Emit(agentevent.RunFailed, runFailedData(code, message, err))
}

// runFailedData 保留对客户端稳定的失败码和文案，同时附带上游失败原因及可识别的 HTTP 状态码。
func runFailedData(code string, message string, err error) agentevent.RunFailedData {
	data := agentevent.RunFailedData{Code: code, Message: message}
	if err == nil {
		return data
	}

	data.Reason = err.Error()
	if matched := httpStatusCodePattern.FindStringSubmatch(data.Reason); len(matched) == 2 {
		data.StatusCode, _ = strconv.Atoi(matched[1])
	}
	return data
}
