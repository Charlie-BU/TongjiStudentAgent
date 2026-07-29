// Package systemtools 提供由 Agent 宿主直接执行的静态系统工具。
package systemtools

import (
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
)

// Tools 返回已通过应用 allowlist 审核的静态系统工具。
func Tools() []tool.BaseTool {
	return buildTools(toolallowlist.IsAllowedTool)
}

// buildTools 构建已通过应用 allowlist 审核的静态系统工具。
func buildTools(isAllowed func(string) bool) []tool.BaseTool {
	registeredTools := make([]tool.BaseTool, 0, 1)
	for _, candidate := range []tool.InvokableTool{newLoadSkillTool(isAllowed)} {
		info, err := candidate.Info(nil)
		if err != nil || info == nil || !isAllowed(info.Name) {
			continue
		}
		registeredTools = append(registeredTools, candidate)
	}
	return registeredTools
}
