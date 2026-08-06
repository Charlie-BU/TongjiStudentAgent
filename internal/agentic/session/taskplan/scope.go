package taskplan

import (
	"context"
	"errors"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
)

type Session = agenticsession.Session
type Persistence = agenticsession.Persistence

const (
	PersistenceDurable   = agenticsession.PersistenceDurable
	PersistenceEphemeral = agenticsession.PersistenceEphemeral
)

var (
	// ErrInvalidTaskPlanScope 表示会话信息不足以限定任务计划访问范围。
	ErrInvalidTaskPlanScope = errors.New("session task plan scope is invalid")
	// ErrTaskPlanScopeUnavailable 表示当前调用不在经过会话授权的 Agent Run 中。
	ErrTaskPlanScopeUnavailable = errors.New("session task plan scope is unavailable")
)

type taskPlanScopeContextKey struct{}
type activeTaskPlanContextKey struct{}

// TaskPlanScope 是由服务层在完成会话归属校验后创建的可信访问范围。
// 字段不导出，模型工具不能通过参数构造或替换其 session 归属。
type TaskPlanScope struct {
	sessionID   string
	ownerUserID string
	persistence Persistence
}

// NewTaskPlanScope 从已验证的会话构造任务计划访问范围。
func NewTaskPlanScope(session Session) (TaskPlanScope, error) {
	if session.ID == "" {
		return TaskPlanScope{}, ErrInvalidTaskPlanScope
	}
	switch session.Persistence {
	case PersistenceDurable:
		if session.OwnerUserID == "" {
			return TaskPlanScope{}, ErrInvalidTaskPlanScope
		}
	case PersistenceEphemeral:
		if session.OwnerUserID != "" {
			return TaskPlanScope{}, ErrInvalidTaskPlanScope
		}
	default:
		return TaskPlanScope{}, ErrInvalidTaskPlanScope
	}
	return TaskPlanScope{sessionID: session.ID, ownerUserID: session.OwnerUserID, persistence: session.Persistence}, nil
}

// WithTaskPlanScope 将当前已授权会话范围传递给同一 Run 中的静态系统工具。
func WithTaskPlanScope(ctx context.Context, scope TaskPlanScope) context.Context {
	return context.WithValue(ctx, taskPlanScopeContextKey{}, scope)
}

// TaskPlanScopeFromContext 返回当前 Run 可信的任务计划访问范围。
func TaskPlanScopeFromContext(ctx context.Context) (TaskPlanScope, bool) {
	scope, ok := ctx.Value(taskPlanScopeContextKey{}).(TaskPlanScope)
	if !ok {
		return TaskPlanScope{}, false
	}
	if _, err := NewTaskPlanScope(Session{ID: scope.sessionID, OwnerUserID: scope.ownerUserID, Persistence: scope.persistence}); err != nil {
		return TaskPlanScope{}, false
	}
	return scope, true
}

// WithActiveTaskPlan 将本轮模型输入使用的活动计划快照写入 context。
func WithActiveTaskPlan(ctx context.Context, plan *TaskPlan) context.Context {
	return context.WithValue(ctx, activeTaskPlanContextKey{}, cloneTaskPlan(plan))
}

// ActiveTaskPlanFromContext 返回本轮模型输入的活动计划快照。
func ActiveTaskPlanFromContext(ctx context.Context) (*TaskPlan, bool) {
	if ctx == nil {
		return nil, false
	}
	plan, ok := ctx.Value(activeTaskPlanContextKey{}).(*TaskPlan)
	if !ok {
		return nil, false
	}
	return cloneTaskPlan(plan), true
}

func cloneTaskPlan(plan *TaskPlan) *TaskPlan {
	if plan == nil {
		return nil
	}
	clone := *plan
	clone.Tasks = append([]TaskItem(nil), plan.Tasks...)
	return &clone
}

// TaskPlanRepository 是 system.manage_task_plan 读写会话级编排状态的唯一入口。
//
// 它不是通用数据库访问对象，只暴露读取 GetTaskPlan、版本化保存 SaveTaskPlan、清理 ClearTaskPlan 三种能力。
// 所有方法从 context 获取由 chat.Service 在会话归属校验后注入的 TaskPlanScope，
// 因而 tool 本身既不接收也无法替换 session_id、owner_user_id 等访问范围；repository 会根据 scope 自动路由到认证会话的 PostgreSQL 或匿名会话的 Redis。
type TaskPlanRepository interface {
	GetTaskPlan(ctx context.Context) (*TaskPlan, error)
	SaveTaskPlan(ctx context.Context, expectedRevision int64, tasks []TaskItem) (TaskPlan, error)
	ClearTaskPlan(ctx context.Context, expectedRevision int64) error
}

type durableTaskPlanStore interface {
	GetTaskPlan(ctx context.Context, sessionID, ownerUserID string) (*TaskPlan, error)
	SaveTaskPlan(ctx context.Context, sessionID, ownerUserID string, expectedRevision int64, tasks []TaskItem) (TaskPlan, error)
	ClearTaskPlan(ctx context.Context, sessionID, ownerUserID string, expectedRevision int64) error
}

type ephemeralTaskPlanStore interface {
	GetTaskPlan(ctx context.Context, sessionID string) (*TaskPlan, error)
	SaveTaskPlan(ctx context.Context, sessionID string, expectedRevision int64, tasks []TaskItem) (TaskPlan, error)
	ClearTaskPlan(ctx context.Context, sessionID string, expectedRevision int64) error
}

type taskPlanRepository struct {
	durable   durableTaskPlanStore
	ephemeral ephemeralTaskPlanStore
}

// NewTaskPlanRepository 将认证与匿名存储收敛为工具可用的 scope-bound repository。
// 与只在单次 Run 内维护内存状态的 system.load_skill 不同，任务计划必须跨请求恢复，
// 因此需要显式注入该持久化访问入口，而不能依赖全局变量或工具参数中的会话标识。
func NewTaskPlanRepository(durable durableTaskPlanStore, ephemeral ephemeralTaskPlanStore) (TaskPlanRepository, error) {
	if durable == nil || ephemeral == nil {
		return nil, errors.New("task plan durable and ephemeral stores are required")
	}
	return &taskPlanRepository{durable: durable, ephemeral: ephemeral}, nil
}

// GetTaskPlan 从存储获取当前会话的任务计划。
func (r *taskPlanRepository) GetTaskPlan(ctx context.Context) (*TaskPlan, error) {
	scope, err := taskPlanScope(ctx)
	if err != nil {
		return nil, err
	}
	switch scope.persistence {
	case PersistenceDurable:
		return r.durable.GetTaskPlan(ctx, scope.sessionID, scope.ownerUserID)
	case PersistenceEphemeral:
		return r.ephemeral.GetTaskPlan(ctx, scope.sessionID)
	default:
		return nil, ErrInvalidTaskPlanScope
	}
}

// SaveTaskPlan 保存当前会话的任务计划。
func (r *taskPlanRepository) SaveTaskPlan(ctx context.Context, expectedRevision int64, tasks []TaskItem) (TaskPlan, error) {
	scope, err := taskPlanScope(ctx)
	if err != nil {
		return TaskPlan{}, err
	}
	switch scope.persistence {
	case PersistenceDurable:
		return r.durable.SaveTaskPlan(ctx, scope.sessionID, scope.ownerUserID, expectedRevision, tasks)
	case PersistenceEphemeral:
		return r.ephemeral.SaveTaskPlan(ctx, scope.sessionID, expectedRevision, tasks)
	default:
		return TaskPlan{}, ErrInvalidTaskPlanScope
	}
}

// ClearTaskPlan 清除当前会话的任务计划。
func (r *taskPlanRepository) ClearTaskPlan(ctx context.Context, expectedRevision int64) error {
	scope, err := taskPlanScope(ctx)
	if err != nil {
		return err
	}
	switch scope.persistence {
	case PersistenceDurable:
		return r.durable.ClearTaskPlan(ctx, scope.sessionID, scope.ownerUserID, expectedRevision)
	case PersistenceEphemeral:
		return r.ephemeral.ClearTaskPlan(ctx, scope.sessionID, expectedRevision)
	default:
		return ErrInvalidTaskPlanScope
	}
}

// taskPlanScope 从 context 中提取任务计划访问范围。
func taskPlanScope(ctx context.Context) (TaskPlanScope, error) {
	if ctx == nil {
		return TaskPlanScope{}, ErrTaskPlanScopeUnavailable
	}
	scope, ok := TaskPlanScopeFromContext(ctx)
	if !ok {
		return TaskPlanScope{}, ErrTaskPlanScopeUnavailable
	}
	return scope, nil
}
