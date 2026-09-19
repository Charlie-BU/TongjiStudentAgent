package mcp

import (
	"encoding/json"
	"strings"

	protocol "github.com/mark3labs/mcp-go/mcp"
)

// 仅保留 MCP 仓固定公开文案，阻止原始上游诊断/凭据通过错误结果进入模型。
var luckinSafeMessages = map[string]struct{}{
	"当前用户尚未绑定瑞幸账号，请先完成瑞幸登录。":              {},
	"无法识别当前同济用户，请先完成同济授权。":                {},
	"无法识别当前同济用户，请重新授权后再绑定瑞幸账号。":           {},
	"暂时无法验证当前同济用户，请稍后重试。":                 {},
	"瑞幸业务参数无效，请检查门店、商品或订单信息。":             {},
	"瑞幸授权无效，请重新完成瑞幸登录。":                   {},
	"瑞幸服务暂时不可用，请稍后重试。":                    {},
	"瑞幸服务返回的数据不完整，无法确认操作成功。":              {},
	"瑞幸未接受验证码发送请求，请核对手机号或稍后重试。":           {},
	"瑞幸未能完成本次操作，请核对业务信息。":                 {},
	"瑞幸未返回完整有效的登录凭据，无法获取 Token，请重新登录。":    {},
	"瑞幸登录态未通过验证，无法获取 Token，请重新登录。":        {},
	"瑞幸登录未成功，请核对手机号和验证码后重试。":              {},
	"瑞幸要求额外安全校验或授权，请先在瑞幸开放平台完成。":          {},
	"瑞幸订单操作结果未确认，请先核实订单状态，不要直接重复创建或取消订单。": {},
	"瑞幸请求超时，结果未确认；请勿立即重复发送验证码或登录。":        {},
	"瑞幸请求超时，请稍后重试。":                       {},
	"瑞幸请求过于频繁，请稍后重试。":                     {},
	"瑞幸返回的数据不完整，无法确认操作结果。":                {},
}

// normalizeNamedMCPToolResult 保留瑞幸成功结果，并按本地白名单保留可公开的业务错误。
func normalizeNamedMCPToolResult(name string, result *protocol.CallToolResult) *protocol.CallToolResult {
	if !strings.HasPrefix(name, "luckin.") {
		// 非瑞幸工具沿用现有归一规则。
		return normalizeMCPToolResult(result)
	}
	if name == "luckin.auth.check" {
		return normalizeLuckinCheckResult(result)
	}
	if result != nil && !result.IsError {
		// 与 Content 重复的顶层 StructuredContent 不再交给 Eino；文本中的业务包装仍保留。
		result.StructuredContent = nil
		return result
	}
	if result != nil {
		for _, content := range result.Content {
			text, ok := toolResultText(content)
			var value struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if !ok || json.Unmarshal([]byte(text), &value) != nil {
				continue
			}
			if value.Status != toolStatusUnauthorized && value.Status != toolStatusUpstreamUnavailable {
				continue
			}
			// 同时限制状态和完整文案；只重建这两个字段，不携带错误中的其他诊断信息。
			if _, safe := luckinSafeMessages[value.Message]; safe {
				data, _ := json.Marshal(value)
				return protocol.NewToolResultText(string(data))
			}
		}
	}
	status := toolStatusExecutionUnavailable
	if result != nil {
		status = toolStatusFromMCPResult(result)
	}
	return protocol.NewToolResultText(namedToolFailureJSON(name, status))
}

// namedToolFailureJSON 用于传输失败或无法识别的 MCP 错误；不执行任何恢复操作。
func namedToolFailureJSON(name, status string) string {
	if name == "luckin.auth.check" {
		if status == toolStatusUpstreamTimeout {
			return luckinCheckErrorJSON("upstream_timeout")
		}
		return luckinCheckErrorJSON("upstream_unavailable")
	}
	if !strings.HasPrefix(name, "luckin.") {
		return stableToolResultJSON(status)
	}
	message := "瑞幸服务暂时不可用，请稍后重试。"
	if status == toolStatusUnauthorized {
		message = "瑞幸授权未能通过，请重新检查登录状态。"
	}
	// 写操作可能已经到达上游，即使客户端没收到结果也不能建议直接重复提交。
	switch name {
	case "luckin.order.create", "luckin.order.cancel":
		message = "瑞幸订单操作结果未确认，请先核实订单状态，不要直接重复创建或取消订单。"
	case "luckin.auth.login", "luckin.auth.send_sms_code":
		message = "瑞幸登录请求结果未确认，请勿自动重复发送验证码或提交登录。"
	}
	data, _ := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{status, message})
	return string(data)
}
