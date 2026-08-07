// Package systemtools 提供由 Agent 宿主直接执行的静态系统工具。
package systemtools

import (
	"context"

	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	managetaskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/managetaskplan"
	searchknowledge "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/searchknowledge"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	"github.com/cloudwego/eino/components/tool"
)

// Option 为静态系统工具提供可选依赖。
type Option func(*options)

type options struct {
	taskPlans       taskplan.TaskPlanRepository
	knowledgeClient *knowledge.Client
}

// WithTaskPlanRepository 注入管理当前会话任务计划所需的 scope-bound repository。
func WithTaskPlanRepository(repository taskplan.TaskPlanRepository) Option {
	return func(options *options) { options.taskPlans = repository }
}

// WithKnowledgeClient 注入已启用的 Ark 知识库客户端。
// nil 表示知识库能力未启用，因此不会向模型注册检索工具。
func WithKnowledgeClient(client *knowledge.Client) Option {
	return func(options *options) { options.knowledgeClient = client }
}

// Tools 返回已通过应用 allowlist 审核的静态系统工具。
func Tools(opts ...Option) []tool.BaseTool {
	options := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return buildTools(toolallowlist.IsAllowedTool, options.taskPlans, options.knowledgeClient)
}

// buildTools 构建已通过应用 allowlist 审核的静态系统工具。
func buildTools(isAllowed func(string) bool, repository taskplan.TaskPlanRepository, knowledgeClient *knowledge.Client) []tool.BaseTool {
	registeredTools := make([]tool.BaseTool, 0, 3)
	candidates := []tool.InvokableTool{loadskill.NewTool(isAllowed)}
	if repository != nil {
		candidates = append(candidates, managetaskplan.NewTool(isAllowed, repository))
	}
	if knowledgeClient != nil {
		candidates = append(candidates, searchknowledge.NewTool(isAllowed, knowledgeClient))
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
