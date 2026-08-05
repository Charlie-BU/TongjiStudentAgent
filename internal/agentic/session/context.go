package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// TurnInput 描述一次模型调用的动态提醒、历史消息和当前用户请求。
type TurnInput struct {
	DynamicReminder *schema.Message
	History         []Message
	UserMessage     *schema.Message
}

// ContextAssembler 将 canonical 会话消息转换为模型输入。
type ContextAssembler struct{}

// NewContextAssembler 创建上下文装配器。
func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{}
}

// AssembleForTurn 按动态提醒、历史消息、当前请求的顺序构造模型输入。
func (a *ContextAssembler) AssembleForTurn(ctx context.Context, input TurnInput) ([]*schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil || !isUserMessage(input.DynamicReminder) || !isUserMessage(input.UserMessage) {
		return nil, ErrInvalidTurnInput
	}

	messages := make([]*schema.Message, 0, len(input.History)+2)
	cacheActive := hasActiveArkResponseCache(input.History, time.Now().Unix())
	if !cacheActive {
		// 无缓存时，直接在插入历史消息前添加动态 reminder
		messages = append(messages, cloneMessage(input.DynamicReminder))
	}
	// 添加历史消息
	var previousSequence int64
	for _, historyMessage := range input.History {
		if historyMessage.Sequence <= previousSequence {
			return nil, fmt.Errorf("%w: history sequence is invalid", ErrInvalidTurnInput)
		}
		if strings.TrimSpace(historyMessage.Content) == "" && len(historyMessage.ToolCalls) == 0 && strings.TrimSpace(historyMessage.ReasoningContent) == "" {
			return nil, fmt.Errorf("%w: history content is invalid", ErrInvalidTurnInput)
		}
		previousSequence = historyMessage.Sequence
		switch historyMessage.Role {
		case MessageRoleUser:
			messages = append(messages, schema.UserMessage(historyMessage.Content))
		case MessageRoleAssistant:
			message := schema.AssistantMessage(historyMessage.Content, historyMessage.ToolCalls)
			message.ReasoningContent = historyMessage.ReasoningContent
			restoreArkResponseCache(message, historyMessage) // 历史 Assistant 消息需要恢复 Ark SDK 能识别的 Extra 字段，用于自动续聊
			messages = append(messages, message)
		case MessageRoleTool:
			if strings.TrimSpace(historyMessage.ToolCallID) == "" {
				return nil, fmt.Errorf("%w: tool call ID is invalid", ErrInvalidTurnInput)
			}
			messages = append(messages, schema.ToolMessage(historyMessage.Content, historyMessage.ToolCallID, schema.WithToolName(historyMessage.ToolName)))
		default:
			return nil, fmt.Errorf("%w: history role is invalid", ErrInvalidTurnInput)
		}
	}
	if cacheActive {
		// 若有缓存消息，Ark SDK 会裁剪最新缓存消息之前的输入，动态 reminder 必须添加在历史消息后。
		messages = append(messages, cloneMessage(input.DynamicReminder))
	}
	// 本轮用户 query
	messages = append(messages, cloneMessage(input.UserMessage))
	return messages, nil
}

// hasActiveArkResponseCache 通过有效期内带 ResponseID 的 Assistant 消息判断是否有活跃的 Ark Response 缓存。
func hasActiveArkResponseCache(history []Message, nowUnix int64) bool {
	for index := len(history) - 1; index >= 0; index-- {
		message := history[index]
		if message.Role == MessageRoleAssistant && strings.TrimSpace(message.ResponseID) != "" && message.ResponseCacheExpiresAt >= nowUnix {
			return true
		}
	}
	return false
}

// restoreArkResponseCache 恢复 Ark Responses API 自动续聊所需的历史元数据。
func restoreArkResponseCache(message *schema.Message, historyMessage Message) {
	if message == nil || strings.TrimSpace(historyMessage.ResponseID) == "" || historyMessage.ResponseCacheExpiresAt <= 0 {
		return
	}
	message.Extra = map[string]any{
		"ark-response-id":              historyMessage.ResponseID,
		"ark-response-cache-expire-at": historyMessage.ResponseCacheExpiresAt,
	}
}

// isUserMessage 判断消息是否为带内容的用户消息。
func isUserMessage(message *schema.Message) bool {
	return message != nil && message.Role == schema.User && strings.TrimSpace(message.Content) != ""
}

// cloneMessage 返回不会与输入共享顶层对象的消息副本。
func cloneMessage(message *schema.Message) *schema.Message {
	clone := *message
	return &clone
}
