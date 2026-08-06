// Package systemtools 提供由 Agent 宿主直接执行的静态系统工具。
package systemtools

import (
	"context"

	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	managetaskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/managetaskplan"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
)

// Option 为静态系统工具提供可选依赖。
type Option func(*options)

type options struct {
	taskPlans taskplan.TaskPlanRepository
}

// WithTaskPlanRepository 注入管理当前会话任务计划所需的 scope-bound repository。
func WithTaskPlanRepository(repository taskplan.TaskPlanRepository) Option {
	return func(options *options) { options.taskPlans = repository }
}

// Tools 返回已通过应用 allowlist 审核的静态系统工具。
func Tools(opts ...Option) []tool.BaseTool {
	options := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return buildTools(toolallowlist.IsAllowedTool, options.taskPlans)
}

// buildTools 构建已通过应用 allowlist 审核的静态系统工具。
func buildTools(isAllowed func(string) bool, repository taskplan.TaskPlanRepository) []tool.BaseTool {
	registeredTools := make([]tool.BaseTool, 0, 2)
	candidates := []tool.InvokableTool{loadskill.NewTool(isAllowed)}
	if repository != nil {
		candidates = append(candidates, managetaskplan.NewTool(isAllowed, repository))
	}
	for _, candidate := range candidates {
		info, err := candidate.Info(context.TODO())
		if err != nil || info == nil || !isAllowed(info.Name) {
			continue
		}
		registeredTools = append(registeredTools, candidate)
	}
	return registeredTools
}
