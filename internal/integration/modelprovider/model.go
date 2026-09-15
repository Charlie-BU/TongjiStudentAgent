// Package modelprovider 按配置创建各档位的模型适配器。
package modelprovider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/arkmodel"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/openroutermodel"
	"github.com/cloudwego/eino/components/model"
)

// NewFromEnv 使用共享供应商配置创建指定档位的模型。
func NewFromEnv(ctx context.Context, modelID, tier string) (model.BaseChatModel, error) {
	effort, ok := map[string]string{"lite": "low", "pro": "medium", "max": "high"}[tier]
	if !ok {
		return nil, fmt.Errorf("model tier must be lite, pro, or max")
	}
	switch strings.TrimSpace(os.Getenv("MODEL_PROVIDER")) {
	case "", "ark":
		return arkmodel.NewFromEnv(ctx, modelID, effort)
	case "openrouter":
		return openroutermodel.NewFromEnv(ctx, modelID, effort)
	default:
		return nil, fmt.Errorf("MODEL_PROVIDER must be ark or openrouter")
	}
}
