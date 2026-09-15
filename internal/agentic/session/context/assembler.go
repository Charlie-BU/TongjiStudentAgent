// Package sessioncontext 将 canonical 会话消息装配为模型输入。
// TODO：待 review
package sessioncontext

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/modelmeta"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	"github.com/cloudwego/eino/schema"
)

type Message = agenticsession.Message
type MessageRole = agenticsession.MessageRole

const (
	MessageRoleUser      = agenticsession.MessageRoleUser
	MessageRoleAssistant = agenticsession.MessageRoleAssistant
	MessageRoleTool      = agenticsession.MessageRoleTool
)

var ErrInvalidTurnInput = agenticsession.ErrInvalidTurnInput

// TurnInput 描述一次模型调用的动态提醒、历史消息和当前用户请求。
type TurnInput struct {
	DynamicReminder *schema.Message
	History         []Message
	UserMessage     *schema.Message
	// Stateless 为 true 时重放完整历史及原始协议数据，不恢复 Ark 响应链元数据。
	// 同时清理不完整工具历史，并将动态提醒放到历史之后以保留稳定前缀。
	Stateless bool
}

// ContextAssembler 将 canonical 会话消息转换为模型输入。
type ContextAssembler struct{}

// NewContextAssembler 创建上下文装配器。
func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{}
}

// AssembleForTurn 根据历史重放策略和 Ark 缓存状态决定动态提醒的位置，当前请求始终放在末尾。
func (a *ContextAssembler) AssembleForTurn(ctx context.Context, input TurnInput) ([]*schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a == nil || !isUserMessage(input.DynamicReminder) || !isUserMessage(input.UserMessage) {
		return nil, ErrInvalidTurnInput
	}

	if input.Stateless {
		// 完整重放不能依赖服务端补齐被截断或失败轮次遗留的工具调用与结果。
		input.History = completeToolHistory(input.History)
	}
	messages := make([]*schema.Message, 0, len(input.History)+2)
	cacheActive := !input.Stateless && hasActiveArkResponseCache(input.History, time.Now().Unix())
	reminderAfterHistory := input.Stateless || cacheActive
	if !reminderAfterHistory {
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
			if !input.Stateless {
				// Ark SDK 需要这些元数据来选择 previous_response_id 并裁剪已缓存的历史。
				restoreArkResponseCache(message, historyMessage)
			}
			// 完整重放需要保留 reasoning 等原始输出项；供应商适配器负责检查来源后再使用。
			if input.Stateless && historyMessage.ProtocolData != "" {
				message.Extra = map[string]any{modelmeta.ProtocolKey: historyMessage.ProtocolData}
			}
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
	if reminderAfterHistory {
		// 完整重放时，把时间、任务计划等动态内容后置，避免它们破坏历史的稳定前缀。
		// Ark 有活跃响应缓存时也需后置，防止 SDK 裁剪已缓存历史时把本轮提醒一起删除。
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
