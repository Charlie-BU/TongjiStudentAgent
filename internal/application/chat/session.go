package chat

import (
	"context"
	"fmt"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
)

// CreateSession 为当前请求创建带名称的会话。
func CreateSession(ctx context.Context, name string) (agenticsession.Session, error) {
	if defaultService == nil {
		return agenticsession.Session{}, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.CreateSession(ctx, name)
}

// ListSessions 返回当前已认证用户的全部持久会话。
func ListSessions(ctx context.Context) ([]agenticsession.Session, error) {
	if defaultService == nil {
		return nil, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.ListSessions(ctx)
}

// RenameSession 修改当前已认证用户拥有的持久会话名称。
func RenameSession(ctx context.Context, sessionID, name string) (agenticsession.Session, error) {
	if defaultService == nil {
		return agenticsession.Session{}, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.RenameSession(ctx, sessionID, name)
}

// DeleteSession 删除当前已认证用户拥有的持久会话。
func DeleteSession(ctx context.Context, sessionID string) error {
	if defaultService == nil {
		return fmt.Errorf("chat service is not initialized")
	}
	return defaultService.DeleteSession(ctx, sessionID)
}

// CreateSession 为当前请求的身份状态创建带名称的会话。
func (s *Service) CreateSession(ctx context.Context, name string) (agenticsession.Session, error) {
	if s == nil {
		return agenticsession.Session{}, fmt.Errorf("chat service is not initialized")
	}
	// userId 存在时，创建持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return agenticsession.Session{}, fmt.Errorf("durable session store is not initialized")
		}
		if creator, ok := s.durableSessionStore.(agenticsession.NamedSessionCreator); ok {
			return creator.CreateWithName(ctx, ownerUserID, name)
		}
		return s.durableSessionStore.Create(ctx, ownerUserID)
	}
	// userId 不存在时，创建临时会话
	if s.ephemeralSessionStore == nil {
		return agenticsession.Session{}, fmt.Errorf("ephemeral session store is not initialized")
	}
	return s.ephemeralSessionStore.Create(ctx)
}

// ListSessions 返回当前已认证用户的全部持久会话。
func (s *Service) ListSessions(ctx context.Context) ([]agenticsession.Session, error) {
	if s == nil || s.durableSessionStore == nil {
		return nil, fmt.Errorf("durable session store is not initialized")
	}
	ownerUserID, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return nil, agenticsession.ErrInvalidOwner
	}
	lister, ok := s.durableSessionStore.(agenticsession.SessionLister)
	if !ok {
		return nil, fmt.Errorf("durable session store does not support listing sessions")
	}
	return lister.List(ctx, ownerUserID)
}

// RenameSession 修改当前已认证用户拥有的持久会话名称。
func (s *Service) RenameSession(ctx context.Context, sessionID, name string) (agenticsession.Session, error) {
	if s == nil || s.durableSessionStore == nil {
		return agenticsession.Session{}, fmt.Errorf("durable session store is not initialized")
	}
	ownerUserID, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return agenticsession.Session{}, agenticsession.ErrInvalidOwner
	}
	renamer, ok := s.durableSessionStore.(agenticsession.SessionRenamer)
	if !ok {
		return agenticsession.Session{}, fmt.Errorf("durable session store does not support renaming sessions")
	}
	return renamer.Rename(ctx, sessionID, ownerUserID, name)
}

// DeleteSession 删除当前已认证用户拥有的持久会话。
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.durableSessionStore == nil {
		return fmt.Errorf("durable session store is not initialized")
	}
	ownerUserID, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return agenticsession.ErrInvalidOwner
	}
	deleter, ok := s.durableSessionStore.(agenticsession.SessionDeleter)
	if !ok {
		return fmt.Errorf("durable session store does not support deleting sessions")
	}
	releaseTurn, err := s.waitForSessionTurn(ctx, sessionID)
	if err != nil {
		return err
	}
	defer releaseTurn()
	return deleter.Delete(ctx, sessionID, ownerUserID)
}
