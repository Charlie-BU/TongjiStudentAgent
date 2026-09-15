package sessioncontext

// completeToolHistory 移除孤立结果和未完成工具链，保留其余有效历史。
func completeToolHistory(history []Message) []Message {
	result := make([]Message, 0, len(history))
	chainStart := 0
	pending := map[string]bool{}
	for _, msg := range history {
		if msg.Role != MessageRoleTool && len(pending) > 0 {
			// 只移除中断的调用及其部分结果，保留之前的对话。
			result = result[:chainStart]
			pending = map[string]bool{}
		}
		switch msg.Role {
		case MessageRoleAssistant:
			chainStart = len(result)
			for _, call := range msg.ToolCalls {
				pending[call.ID] = true
			}
		case MessageRoleTool:
			if !pending[msg.ToolCallID] {
				continue
			} else {
				delete(pending, msg.ToolCallID)
			}
		}
		result = append(result, msg)
	}
	if len(pending) > 0 {
		result = result[:chainStart]
	}
	return result
}
