// MCP isError 结果归一、稳定状态和安全消息生成。
package mcp

import (
	"encoding/json"
	"errors"
	"net"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	toolStatusUnauthorized         = "unauthorized"
	toolStatusUpstreamTimeout      = "upstream_timeout"
	toolStatusUpstreamUnavailable  = "upstream_unavailable"
	toolStatusExecutionUnavailable = "tool_execution_failed"
)

// normalizeMCPToolResult 将 MCP Server 的业务错误转换为 Agent 可安全消费的稳定结果。
func normalizeMCPToolResult(result *mcp.CallToolResult) *mcp.CallToolResult {
	if result == nil {
		return stableMCPToolResult(toolStatusExecutionUnavailable)
	}
	if !result.IsError {
		return result
	}
	return stableMCPToolResult(toolStatusFromMCPResult(result))
}

// toolStatusFromMCPResult 从受信任的 MCP 业务错误中提取允许公开的状态。
func toolStatusFromMCPResult(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		text, ok := toolResultText(content)
		if !ok {
			continue
		}
		var payload struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			continue
		}
		switch payload.Status {
		case toolStatusUnauthorized, toolStatusUpstreamTimeout, toolStatusUpstreamUnavailable:
			return payload.Status
		}
	}
	return toolStatusExecutionUnavailable
}

// toolResultText 读取 MCP 文本内容，忽略图片、音频等不应作为错误码来源的内容。
func toolResultText(content mcp.Content) (string, bool) {
	switch value := content.(type) {
	case mcp.TextContent:
		return value.Text, true
	case *mcp.TextContent:
		return value.Text, value != nil
	default:
		return "", false
	}
}

// toolStatusForInvocationError 将远程调用的传输错误映射为稳定状态。
func toolStatusForInvocationError(err error) string {
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return toolStatusUpstreamTimeout
	}
	return toolStatusUpstreamUnavailable
}

// stableMCPToolResult 生成不会携带上游原始内容的 MCP Tool Result。
func stableMCPToolResult(status string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: stableToolResultJSON(status)}},
	}
}

// stableToolResultJSON 返回供 Agent 消费的稳定工具结果。
func stableToolResultJSON(status string) string {
	message := "校园服务暂时不可用，请稍后重试。"
	switch status {
	case toolStatusUnauthorized:
		message = "同济账号授权无效或已过期，请重新完成授权后再试。"
	case toolStatusUpstreamTimeout:
		message = "校园服务响应超时，请稍后重试。"
	case toolStatusUpstreamUnavailable:
		message = "校园服务暂时不可用，请稍后重试。"
	case toolStatusExecutionUnavailable:
		message = "校园工具执行失败，请稍后重试。"
	}
	result, err := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{Status: status, Message: message})
	if err != nil {
		return `{"status":"tool_execution_failed","message":"校园工具执行失败，请稍后重试。"}`
	}
	return string(result)
}
