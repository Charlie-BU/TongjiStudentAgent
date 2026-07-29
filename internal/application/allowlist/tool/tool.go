// Package tool 集中维护允许调用的 tool 白名单。
package tool

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// 静态系统 Tool
	LoadSkillTool = "system.load_skill"
	// 远程 MCP Tool
	TongjiStudentScoreTool = "tongji.student.score"
)

var (
	allowedSystemTools = []string{
		LoadSkillTool,
	}

	allowedMCPTools = []string{
		TongjiStudentScoreTool,
	}
)

// SystemTools 返回已批准 Tool 名称的副本，调用方修改结果不会影响 allowlist。
func SystemTools() []string {
	return append([]string(nil), allowedSystemTools...)
}

// MCPTools 返回已批准 Tool 名称的副本，调用方修改结果不会影响 allowlist。
func MCPTools() []string {
	return append([]string(nil), allowedMCPTools...)
}

// IsAllowedTool 判断 Tool 名称是否已被应用 allowlist 明确批准。
func IsAllowedTool(toolName string) bool {
	for _, allowedTool := range append(allowedSystemTools, allowedMCPTools...) {
		if toolName == allowedTool {
			return true
		}
	}
	return false
}

// ValidateToolAllowlist 确保远程 Tool 只能由非空且无重复的 allowlist 注册。
func ValidateToolAllowlist(toolNames []string) error {
	if len(toolNames) == 0 {
		return errors.New("Tool allowlist cannot be empty")
	}
	seen := make(map[string]struct{}, len(toolNames))
	for _, toolName := range toolNames {
		if strings.TrimSpace(toolName) == "" {
			return errors.New("Tool allowlist cannot contain an empty tool name")
		}
		if _, exists := seen[toolName]; exists {
			return fmt.Errorf("Tool allowlist contains duplicate tool %q", toolName)
		}
		seen[toolName] = struct{}{}
	}
	return nil
}
