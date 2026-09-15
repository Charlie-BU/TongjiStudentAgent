package chat

import (
	"context"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/runtime"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	sessionconfig "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/config"
	sessionpostgres "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/postgres"
	sessionredis "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/redis"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/webfetch"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

// sessionRuntime 描述会话执行链路所需的最小运行时能力。
type sessionRuntime interface {
	StreamWithHistoryAndMessages(ctx context.Context, query, studentInfo string, history []agenticsession.Message, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error)
}

// Service 组装聊天、会话 Runtime 与外部适配器。
type Service struct {
	closeResources func() error // 初始化成功后接管同一组资源的释放

	runtimes              map[string]modelRuntime       // 模型 tier 映射的运行时环境表
	tongjiClient          *tongjiapi.Client             // 同济开放平台客户端
	durableSessionStore   agenticsession.Store          // 认证会话存储
	ephemeralSessionStore agenticsession.EphemeralStore // 匿名会话存储
	turnLocker            agenticsession.TurnLocker     // 会话执行锁
	taskPlanRepository    taskplan.TaskPlanRepository   // 当前会话任务计划
	historyMessageLimit   int                           // 上下文历史消息上限
}

// initializationDeps 隔离启动阶段的外部依赖，不使用可变的全局测试钩子。
type initializationDeps struct {
	instruction    func(context.Context) (string, error)
	model          func(context.Context, string, string) (model.BaseChatModel, error)
	knowledge      func() (*knowledge.Client, error)
	tongji         func() (*tongjiapi.Client, error)
	catalog        func() (string, error)
	tavily         func() (*tavily.Client, error)
	webfetch       func() (*webfetch.Client, error)
	verifyChromium func(context.Context) error
	mcp            func(context.Context) (*mcpclient.Client, error)
	mcpTools       func(context.Context, *mcpclient.Client, ...string) ([]tool.BaseTool, error)
	sandboxEnabled func() (bool, error)
	middleware     func(context.Context) (adk.ChatModelAgentMiddleware, error)
	sessionConfig  func() (sessionconfig.Config, error)
	postgres       func(context.Context) (*sessionpostgres.PostgresStore, error)
	schema         func(context.Context, *sessionpostgres.PostgresStore) error
	redis          func(context.Context, time.Duration, int) (*sessionredis.RedisEphemeralStore, error)
	taskplan       func(*sessionpostgres.PostgresStore, *sessionredis.RedisEphemeralStore) (taskplan.TaskPlanRepository, error)
	runtime        func(context.Context, runtime.Config) (sessionRuntime, error)
	closeMCP       func(*mcpclient.Client) error
	closePostgres  func(*sessionpostgres.PostgresStore)
	closeRedis     func(*sessionredis.RedisEphemeralStore) error
}

type modelRuntime struct {
	runtime sessionRuntime
	modelID string
}
type modelSelectionKey struct{}
type modelSelection struct{ tier, modelID string }
