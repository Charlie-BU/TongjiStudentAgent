package chat

import (
	"context"
	"fmt"

	promptallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/prompt"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/cozeloop"
	"github.com/cloudwego/eino/schema"
)

// loadSystemInstruction 从 Cozeloop PromptHub 加载 system prompt。
func loadSystemInstruction(ctx context.Context) (string, error) {
	if !cozeloop.Enabled() {
		return "", nil
	}

	messages, err := cozeloop.FetchPrompt(ctx, promptallowlist.TongjiStudentSystemPrompt, "", nil)
	if err != nil {
		return "", fmt.Errorf("load system prompt: %w", err)
	}

	instruction, err := cozeloop.MessageContent(messages, schema.System)
	if err != nil {
		return "", fmt.Errorf("system prompt %q: %w", promptallowlist.TongjiStudentSystemPrompt, err)
	}
	return instruction, nil
}
