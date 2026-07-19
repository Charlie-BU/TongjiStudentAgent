// Package arkmodel 提供 Ark ChatModel 适配。
package arkmodel

import (
	"context"
	"fmt"
	"os"

	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/eino-ext/components/model/ark"
)

// NewFromEnv 根据模型环境变量创建 Ark ChatModel。
func NewFromEnv(ctx context.Context) (*ark.ChatModel, error) {
	endpointID := os.Getenv("ENDPOINT_ID")
	endpointAPIKey := os.Getenv("ENDPOINT_API_KEY")
	if endpointID == "" || endpointAPIKey == "" {
		return nil, fmt.Errorf("ENDPOINT_ID or ENDPOINT_API_KEY is not set")
	}

	arkBaseURL := os.Getenv("ARK_BASE_URL")
	if arkBaseURL == "" {
		arkBaseURL = os.Getenv("ARK_BASE_URL_CN")
	}
	if arkBaseURL == "" {
		return nil, fmt.Errorf("ARK_BASE_URL or ARK_BASE_URL_CN is not set")
	}

	logs.Infof("about to initialize model with endpoint id: %s, base url: %s", endpointID, arkBaseURL)
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		Model:   endpointID,
		APIKey:  endpointAPIKey,
		BaseURL: arkBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ark chat model: %w", err)
	}
	return chatModel, nil
}
