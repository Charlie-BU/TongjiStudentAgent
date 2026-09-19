package chat

import (
	"context"
	"fmt"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

// ListSessionMessages 读取当前请求可访问会话的固定快照分页。
func ListSessionMessages(ctx context.Context, sessionID string, limit int, offset int, snapshotSequence int64) (agenticsession.MessagePage, error) {
	if defaultService == nil {
		return agenticsession.MessagePage{}, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.ListSessionMessages(ctx, sessionID, limit, offset, snapshotSequence)
}

// ListSessionMessages 读取当前请求可访问会话的固定快照分页。
func (s *Service) ListSessionMessages(ctx context.Context, sessionID string, limit, offset int, snapshotSequence int64) (agenticsession.MessagePage, error) {
	if s == nil {
		return agenticsession.MessagePage{}, fmt.Errorf("chat service is not initialized")
	}
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		paginator, supported := s.durableSessionStore.(agenticsession.MessagePaginator)
		if !supported {
			return agenticsession.MessagePage{}, fmt.Errorf("durable session store does not support message pagination")
		}
		return paginator.ListMessagePage(ctx, sessionID, ownerUserID, limit, offset, snapshotSequence)
	}
	paginator, supported := s.ephemeralSessionStore.(agenticsession.EphemeralMessagePaginator)
	if !supported {
		return agenticsession.MessagePage{}, fmt.Errorf("ephemeral session store does not support message pagination")
	}
	return paginator.ListMessagePage(ctx, sessionID, limit, offset, snapshotSequence)
}

// sessionTurnOperations 为本轮选择存储、读取历史并构造用户消息追加操作。
// 返回会话历史、追加用户消息函数和错误。
func (s *Service) sessionTurnOperations(ctx context.Context, sessionID, query, runID string) ([]agenticsession.Message, func() (agenticsession.AppendResult, error), error) {
	if s == nil {
		return nil, nil, fmt.Errorf("chat service is not initialized")
	}
	// userId 存在时，选择持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return nil, nil, fmt.Errorf("durable session store is not initialized")
		}
		history, err := s.durableSessionStore.ListMessages(ctx, sessionID, ownerUserID, s.historyLimit())
		if err != nil {
			return nil, nil, err
		}
		return history,
			func() (agenticsession.AppendResult, error) {
				return s.durableSessionStore.Append(ctx, sessionID, ownerUserID, withModelMetadata(ctx, agenticsession.NewMessage{RunID: runID, Role: agenticsession.MessageRoleUser, Content: query}))
			}, nil
	}
	// userId 不存在时，选择临时会话
	if s.ephemeralSessionStore == nil {
		return nil, nil, fmt.Errorf("ephemeral session store is not initialized")
	}
	history, err := s.ephemeralSessionStore.ListMessages(ctx, sessionID, s.historyLimit())
	if err != nil {
		return nil, nil, err
	}
	return history,
		func() (agenticsession.AppendResult, error) {
			return s.ephemeralSessionStore.Append(ctx, sessionID, withModelMetadata(ctx, agenticsession.NewMessage{RunID: runID, Role: agenticsession.MessageRoleUser, Content: query}))
		}, nil
}

// appendAgentMessage 追加 Agent 输出消息到会话。
// 参数：上下文、sessionID、本轮 runID、Agent 消息
// 返回：错误信息
func (s *Service) appendAgentMessage(ctx context.Context, sessionID, runID string, message *schema.Message) error {
	input, err := agenticsession.NewMessageFromSchema(message)
	if err != nil {
		return err
	}
	input.RunID = runID
	input = withModelMetadata(ctx, input)
	input.ResponseID, _ = ark.GetResponseID(message)
	input.ResponseCacheExpiresAt, _ = ark.GetCacheExpiration(message)
	// userId 存在时，选择持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		_, err = s.durableSessionStore.Append(ctx, sessionID, ownerUserID, input)
		return err
	}
	// userId 不存在时，选择临时会话
	_, err = s.ephemeralSessionStore.Append(ctx, sessionID, input)
	return err
}

// historyLimit 返回有效的上下文历史消息数量。
func (s *Service) historyLimit() int {
	if s.historyMessageLimit > 0 {
		return s.historyMessageLimit
	}
	return 100
}
