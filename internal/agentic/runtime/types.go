package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	knowledgeModel "github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

type knowledgeDocumentsProvider interface {
	ListDocs(context.Context, knowledgeModel.ListDocsRequest) (*knowledgeModel.ListDocsResponse, error)
}

// Config 描述运行时所需的通用 Agent 依赖。
type Config struct {
	Name            string
	Description     string
	Instruction     string
	SkillCatalog    string
	KnowledgeClient knowledgeDocumentsProvider
	ChatModel       model.BaseChatModel
	Tools           []tool.BaseTool
	MaxIterations   int
	Handlers        []adk.ChatModelAgentMiddleware
}

// Runtime 持有已初始化的 DeepAgent。
type Runtime struct {
	stateless       bool // 使用完整历史重放策略；不代表禁用会话存储或前缀计算缓存
	agent           adk.Agent
	skillCatalog    string
	knowledgeClient knowledgeDocumentsProvider
}
