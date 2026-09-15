package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
)

const deleteSessionTurnRetryInterval = 10 * time.Millisecond

// acquireSessionTurn 为会话获取执行锁，确保并发安全。
func (s *Service) acquireSessionTurn(ctx context.Context, sessionID string) (agenticsession.TurnRelease, error) {
	if s == nil || s.turnLocker == nil {
		return nil, fmt.Errorf("session turn locker is not initialized")
	}
	return s.turnLocker.AcquireTurn(ctx, sessionID)
}

// waitForSessionTurn 等待正在执行的会话轮次结束后获取执行锁。
func (s *Service) waitForSessionTurn(ctx context.Context, sessionID string) (agenticsession.TurnRelease, error) {
	for {
		releaseTurn, err := s.acquireSessionTurn(ctx, sessionID)
		if !errors.Is(err, agenticsession.ErrTurnInProgress) {
			return releaseTurn, err
		}
		timer := time.NewTimer(deleteSessionTurnRetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
