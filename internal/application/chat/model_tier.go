package chat

import (
	"context"
	"errors"
	"fmt"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
)

var ErrInvalidModelTier = errors.New("model_tier must be lite, pro, or max")
var ErrModelTierUnavailable = errors.New("model tier is not configured")

// resolveModelTier 根据模型等级返回对应的运行时环境。
func (s *Service) resolveModelTier(tier string) (modelRuntime, error) {
	switch tier {
	case "lite", "pro", "max":
	default:
		// 不在范围内，返回错误
		return modelRuntime{}, ErrInvalidModelTier
	}
	if s != nil {
		if selected, ok := s.runtimes[tier]; ok && selected.runtime != nil {
			return selected, nil
		}
	}
	return modelRuntime{}, fmt.Errorf("%w: %s", ErrModelTierUnavailable, tier)
}

// ValidateModelTier 校验模型等级是否有效。
// 在处理消息前调用，确保模型等级配置正确。
func ValidateModelTier(tier string) error {
	_, err := defaultService.resolveModelTier(tier)
	return err
}

// withModelMetadata 为 input 从上下文添加模型元数据。
func withModelMetadata(ctx context.Context, input agenticsession.NewMessage) agenticsession.NewMessage {
	if selection, ok := ctx.Value(modelSelectionKey{}).(modelSelection); ok {
		input.ModelTier, input.ModelID = selection.tier, selection.modelID
	}
	return input
}

// sanitizeHistoryForModel 清理当前模型连续会话段之前的协议数据与响应缓存信息。
// 用于在模型变更时，确保缓存的响应仅用于当前模型。
func sanitizeHistoryForModel(history []agenticsession.Message, modelID string) []agenticsession.Message {
	result := append([]agenticsession.Message(nil), history...)
	boundary := -1 // 切换分界点
	for i, m := range result {
		if modelID == "" || m.ModelID != modelID {
			boundary = i
		}
	}
	for i := range result {
		if i <= boundary {
			result[i].ProtocolData = ""
			result[i].ResponseID = ""
			result[i].ResponseCacheExpiresAt = 0
		}
	}
	return result
}
