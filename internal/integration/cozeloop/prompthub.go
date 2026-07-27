package cozeloop

import (
	"context"
	"fmt"
	"strings"

	cozeloopprompt "github.com/cloudwego/eino-ext/components/prompt/cozeloop"
	"github.com/cloudwego/eino/schema"
)

// FetchPrompt 拉取指定版本的 PromptHub 提示词，并使用 variables 动态格式化。
// promptVersion 为空时拉取最新版本。
func FetchPrompt(ctx context.Context, promptKey string, promptVersion string, variables map[string]any) ([]*schema.Message, error) {
	if client == nil {
		return nil, fmt.Errorf("Cozeloop client is not initialized")
	}

	ph, err := cozeloopprompt.NewPromptHub(ctx, &cozeloopprompt.Config{
		Key:            promptKey,
		Version:        promptVersion,
		CozeLoopClient: client,
	})
	if err != nil {
		return nil, fmt.Errorf("create prompt hub: %w", err)
	}

	messages, err := ph.Format(ctx, variables)
	if err != nil {
		return nil, fmt.Errorf("format prompt %q: %w", promptKey, err)
	}
	return messages, nil
}

// MessageContent 提取指定角色的文本消息内容并按顺序拼接。
func MessageContent(messages []*schema.Message, role schema.RoleType) (string, error) {
	var contents []string
	for _, message := range messages {
		if message == nil || message.Role != role || strings.TrimSpace(message.Content) == "" {
			continue
		}
		contents = append(contents, strings.TrimSpace(message.Content))
	}
	if len(contents) == 0 {
		return "", fmt.Errorf("prompt contains no %q message", role)
	}
	return strings.Join(contents, "\n\n"), nil
}
