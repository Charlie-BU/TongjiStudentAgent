// Package openroutermodel 提供 OpenRouter Responses 模型工厂。
package openroutermodel

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/model"
)

// NewFromEnv 使用共享凭据创建指定档位模型。
func NewFromEnv(ctx context.Context, modelID, effort string) (model.BaseChatModel, error) {
	if effort != "low" && effort != "medium" && effort != "high" {
		return nil, fmt.Errorf("reasoning effort must be low, medium, or high")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return newModel(config{APIKey: os.Getenv("OPENROUTER_API_KEY"), BaseURL: os.Getenv("OPENROUTER_BASE_URL"), Model: modelID, Effort: effort})
}
