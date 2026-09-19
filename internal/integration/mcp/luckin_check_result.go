package mcp

import (
	"encoding/json"
	protocol "github.com/mark3labs/mcp-go/mcp"
)

// 按已知错误码重建公开提示，不透传远端原始 message，也不将检查失败误报为未登录。
var luckinCheckMessages = map[string]string{
	"platform_unauthorized":        "无法识别当前同济用户，请先完成或更新同济授权，无需重新登录瑞幸。",
	"platform_unavailable":         "同济身份服务暂时不可用，暂时无法检查瑞幸登录，请稍后重试。",
	"upstream_timeout":             "瑞幸登录检查超时，请稍后重试；当前不能判断 Token 是否有效。",
	"rate_limited":                 "瑞幸登录检查过于频繁，请稍后重试，不要重新发送验证码。",
	"upstream_unavailable":         "瑞幸服务暂时不可用或响应异常，暂时无法检查登录状态，请稍后重试。",
	"upstream_forbidden":           "瑞幸拒绝访问，暂时无法确认登录状态，请稍后重试或联系平台维护方。",
	"credential_store_unavailable": "瑞幸凭据存储暂时不可用，请稍后重试或联系平台维护方。",
	"credential_changed":           "瑞幸登录信息在检查期间发生变化，请重新检查登录状态。",
}

func luckinCheckErrorJSON(status string) string {
	message, ok := luckinCheckMessages[status]
	if !ok {
		status = "upstream_unavailable"
		message = luckinCheckMessages[status]
	}
	result, _ := json.Marshal(struct {
		Valid   bool   `json:"valid"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}{false, status, message})
	return string(result)
}
func normalizeLuckinCheckResult(result *protocol.CallToolResult) *protocol.CallToolResult {
	if result != nil {
		for _, content := range result.Content {
			text, ok := toolResultText(content)
			if !ok {
				continue
			}
			var value struct {
				Valid  *bool  `json:"valid"`
				Status string `json:"status"`
			}
			if json.Unmarshal([]byte(text), &value) != nil {
				continue
			}
			if _, known := luckinCheckMessages[value.Status]; known {
				return protocol.NewToolResultText(luckinCheckErrorJSON(value.Status))
			}
			if !result.IsError && value.Status == "" && value.Valid != nil {
				message := "尚未登录瑞幸或登录已失效，请完成瑞幸登录。"
				if *value.Valid {
					message = "瑞幸登录有效，可以继续操作。"
				}
				data, _ := json.Marshal(struct {
					Valid   bool   `json:"valid"`
					Message string `json:"message"`
				}{*value.Valid, message})
				return protocol.NewToolResultText(string(data))
			}
		}
	}
	return protocol.NewToolResultText(luckinCheckErrorJSON("upstream_unavailable"))
}
