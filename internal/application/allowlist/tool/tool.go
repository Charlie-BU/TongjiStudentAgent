// Package tool 集中维护允许调用的 tool 白名单。
package tool

import (
	"errors"
	"fmt"
	"strings"
)

const (
	TongjiStudentScoreTool = "tongji.student.score"
)

var (
	mcpTools = []string{TongjiStudentScoreTool}
)

// MCPTools 返回允许注册到 Agent 的远程 MCP Tool 名称。
func MCPTools() []string {
	return append([]string(nil), mcpTools...)
}

// ValidateToolAllowlist 确保远程 MCP Tool 只能由非空且无重复的 allowlist 注册。
func ValidateToolAllowlist(toolNames []string) error {
	if len(toolNames) == 0 {
		return errors.New("MCP tool allowlist cannot be empty")
	}
	seen := make(map[string]struct{}, len(toolNames))
	for _, toolName := range toolNames {
		if strings.TrimSpace(toolName) == "" {
			return errors.New("MCP tool allowlist cannot contain an empty tool name")
		}
		if _, exists := seen[toolName]; exists {
			return fmt.Errorf("MCP tool allowlist contains duplicate tool %q", toolName)
		}
		seen[toolName] = struct{}{}
	}
	return nil
}
