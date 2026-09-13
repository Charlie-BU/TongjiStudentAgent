package chat

import (
	"context"
	"errors"
	"fmt"

	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
)

var ErrInvalidModelTier = errors.New("model_tier must be lite, pro, or max")
var ErrModelTierUnavailable = errors.New("model tier is not configured")

type modelRuntime struct {
	runtime sessionRuntime
	modelID string
}
type modelSelectionKey struct{}
type modelSelection struct{ tier, modelID string }

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
		// Support explicitly injected single-runtime services.
		if s.runtimes == nil && tier == "lite" && s.runtime != nil {
			return modelRuntime{runtime: s.runtime}, nil
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

// withModelMetadata 为上下文添加模型元数据。
func withModelMetadata(ctx context.Context, input agenticsession.NewMessage) agenticsession.NewMessage {
	if selection, ok := ctx.Value(modelSelectionKey{}).(modelSelection); ok {
		input.ModelTier, input.ModelID = selection.tier, selection.modelID
	}
	return input
}

// historyForModel 从历史记录中提取指定模型的连续后缀。
// 用于在模型等级变更时，确保缓存的响应仅用于当前模型。
func historyForModel(history []agenticsession.Message, modelID string) []agenticsession.Message {
	result := append([]agenticsession.Message(nil), history...)
	boundary := -1 // 切换分界点
	for i, m := range result {
		if modelID == "" || m.ModelID != modelID {
			boundary = i
		}
	}
	for i := range result {
		if i <= boundary {
			result[i].ResponseID = ""
			result[i].ResponseCacheExpiresAt = 0
		}
	}
	return result
}
