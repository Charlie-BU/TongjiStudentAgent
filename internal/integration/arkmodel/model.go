// Package arkmodel 提供 Ark ChatModel 适配。
package arkmodel

import (
	"context"
	"fmt"
	"os"

	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	arkruntimeModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// responseChainCacheTTLSeconds 响应缓存 TTL，单位秒。
const responseChainCacheTTLSeconds = 600

// NewFromEnv 根据模型环境变量创建 Ark ChatModel。
func NewFromEnv(ctx context.Context) (model.BaseChatModel, error) {
	liteModel := os.Getenv("LITE_MODEL")
	arkAPIKey := os.Getenv("ARK_API_KEY")
	if liteModel == "" || arkAPIKey == "" {
		return nil, fmt.Errorf("LITE_MODEL or ARK_API_KEY is not set")
	}

	arkBaseURL := os.Getenv("ARK_BASE_URL")
	if arkBaseURL == "" {
		arkBaseURL = os.Getenv("ARK_BASE_URL_CN")
	}
	if arkBaseURL == "" {
		return nil, fmt.Errorf("ARK_BASE_URL or ARK_BASE_URL_CN is not set")
	}

	reasoningEffort := arkruntimeModel.ReasoningEffortMedium
	logs.Infof("about to initialize model with model: %s, base url: %s", liteModel, arkBaseURL)
	chatModel, err := ark.NewResponsesAPIChatModel(ctx, &ark.ResponsesAPIConfig{
		Model:           liteModel,
		APIKey:          arkAPIKey,
		BaseURL:         arkBaseURL,
		ReasoningEffort: &reasoningEffort,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         responseChainCacheTTLSeconds,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Ark Responses API chat model: %w", err)
	}
	return chatModel, nil
}
