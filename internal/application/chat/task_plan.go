package chat

import (
	"context"
	"fmt"

	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
)

// GetSessionTaskPlan 读取当前请求有权访问的会话任务计划。
func GetSessionTaskPlan(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error) {
	if defaultService == nil {
		return nil, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.GetSessionTaskPlan(ctx, sessionID)
}

// GetSessionTaskPlan 读取当前请求有权访问的会话任务计划。
func (s *Service) GetSessionTaskPlan(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error) {
	if s == nil || s.taskPlanRepository == nil {
		return nil, fmt.Errorf("task plan repository is not initialized")
	}
	scope, err := s.taskPlanScope(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ctx = taskplan.WithTaskPlanScope(ctx, scope)
	return s.taskPlanRepository.GetTaskPlan(ctx)
}

// taskPlanScope 在 Runtime 启动前再次读取已授权 session，并据此创建 TaskPlanScope。
func (s *Service) taskPlanScope(ctx context.Context, sessionID string) (taskplan.TaskPlanScope, error) {
	if s == nil {
		return taskplan.TaskPlanScope{}, fmt.Errorf("chat service is not initialized")
	}
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return taskplan.TaskPlanScope{}, fmt.Errorf("durable session store is not initialized")
		}
		session, err := s.durableSessionStore.Get(ctx, sessionID, ownerUserID)
		if err != nil {
			return taskplan.TaskPlanScope{}, err
		}
		return taskplan.NewTaskPlanScope(session)
	}
	if s.ephemeralSessionStore == nil {
		return taskplan.TaskPlanScope{}, fmt.Errorf("ephemeral session store is not initialized")
	}
	session, err := s.ephemeralSessionStore.Get(ctx, sessionID)
	if err != nil {
		return taskplan.TaskPlanScope{}, err
	}
	return taskplan.NewTaskPlanScope(session)
}
