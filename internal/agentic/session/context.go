package session

import (
	"context"
	"fmt"
	"strings"

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
	// 系统动态 reminder
	messages = append(messages, cloneMessage(input.DynamicReminder))
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
	// 本轮用户 query
	messages = append(messages, cloneMessage(input.UserMessage))
	return messages, nil
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
