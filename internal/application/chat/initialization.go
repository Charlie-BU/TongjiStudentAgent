package chat

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/runtime"
	sessionconfig "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/config"
	sessionpostgres "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/postgres"
	sessionredis "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/redis"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	mcpintegration "github.com/Charlie-BU/TongjiStudent/internal/integration/mcp"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/modelprovider"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/sandbox"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/webfetch"
	"github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
	"github.com/cloudwego/eino/adk"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

// initialize 组装服务依赖，并在失败时释放已获得的资源。
func (deps initializationDeps) initialize(ctx context.Context) (*Service, error) {
	// 1. 准备 Agent 的静态输入。它们不持有需要在本函数中释放的连接。
	// 系统提示词
	instruction, err := deps.instruction(ctx)
	if err != nil {
		return nil, err
	}
	// 模型
	liteModelID := os.Getenv("LITE_MODEL")
	chatModel, err := deps.model(ctx, liteModelID, "lite")
	if err != nil {
		return nil, fmt.Errorf("initialize chat model: %w", err)
	}
	// 知识库
	knowledgeClient, err := deps.knowledge()
	if err != nil {
		return nil, fmt.Errorf("initialize knowledge client: %w", err)
	}
	tongjiClient, err := deps.tongji()
	if err != nil {
		return nil, fmt.Errorf("initialize Tongji Open Platform client: %w", err)
	}
	if tongjiClient == nil {
		return nil, fmt.Errorf("Tongji Open Platform client is not initialized")
	}
	// skills 目录
	skillCatalog, err := deps.catalog()
	if err != nil {
		return nil, fmt.Errorf("build skill catalog: %w", err)
	}

	// 2. 初始化公开网页能力，浏览器不可用时仅启用 HTTP 提取。
	tavilyClient, err := deps.tavily()
	if err != nil {
		return nil, fmt.Errorf("initialize Tavily client: %w", err)
	}
	webFetchClient, err := deps.webfetch()
	if err != nil {
		return nil, fmt.Errorf("initialize adaptive web fetch client: %w", err)
	}
	if err := deps.verifyChromium(ctx); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		webFetchClient.DisableBrowser()
		logging.Warnf("Chromium unavailable; browser extraction disabled, url_fetch will use HTTP only: %v", err)
	}

	// 3. 建立远程 MCP 连接。此后的失败路径统一释放已获得的资源。
	mcpClient, err := deps.mcp(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize remote mcp client: %w", err)
	}
	var postgresStore *sessionpostgres.PostgresStore
	var redisStore *sessionredis.RedisEphemeralStore
	initialized := false
	closeResources := func() error {
		var closeErr error
		if redisStore != nil {
			closeErr = errors.Join(closeErr, deps.closeRedis(redisStore))
		}
		if postgresStore != nil {
			deps.closePostgres(postgresStore)
		}
		closeErr = errors.Join(closeErr, deps.closeMCP(mcpClient))
		return closeErr
	}
	defer func() {
		if !initialized {
			_ = closeResources()
		}
	}()

	mcpTools, err := deps.mcpTools(ctx, mcpClient, toolallowlist.MCPTools()...)
	if err != nil {
		return nil, fmt.Errorf("convert mcp tools: %w", err)
	}

	// 4. 仅在启用时添加沙箱中间件，避免无配置时引入额外依赖。
	handlers := make([]adk.ChatModelAgentMiddleware, 0, 1)
	sandboxEnabled, err := deps.sandboxEnabled()
	if err != nil {
		return nil, fmt.Errorf("read sandbox configuration: %w", err)
	}
	if sandboxEnabled {
		filesystemMiddleware, err := deps.middleware(ctx)
		if err != nil {
			return nil, fmt.Errorf("create filesystem middleware: %w", err)
		}
		handlers = append(handlers, filesystemMiddleware)
	}

	// 5. 初始化会话持久化与任务计划。任务计划依赖 PostgreSQL 和 Redis。
	sessionConfig, err := deps.sessionConfig()
	if err != nil {
		return nil, fmt.Errorf("read session configuration: %w", err)
	}
	postgresStore, err = deps.postgres(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL session store: %w", err)
	}
	if err := deps.schema(ctx, postgresStore); err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL session schema: %w", err)
	}
	redisStore, err = deps.redis(ctx, sessionConfig.AnonymousTTL, sessionConfig.AnonymousMessageLimit)
	if err != nil {
		return nil, fmt.Errorf("initialize Redis session store: %w", err)
	}
	taskPlanRepository, err := deps.taskplan(postgresStore, redisStore)
	if err != nil {
		return nil, fmt.Errorf("initialize task plan repository: %w", err)
	}

	// 6. 将所有已验证的能力组装为工具集合和 Agent Runtime。
	tools := append(systemtools.Tools(
		systemtools.WithTaskPlanRepository(taskPlanRepository),
		systemtools.WithKnowledgeClient(knowledgeClient),
		systemtools.WithTavilyClient(tavilyClient),
		systemtools.WithWebFetchClient(webFetchClient),
	), mcpTools...)

	// Lite model runtime
	runtimeConfig := runtime.Config{
		Name:            "Tongji Student Agent",
		Description:     "Campus assistant that answers questions using approved Tongji services.",
		Instruction:     instruction,
		SkillCatalog:    skillCatalog,
		KnowledgeClient: knowledgeClient,
		ChatModel:       chatModel,
		Tools:           tools,
		MaxIterations:   20,
		Handlers:        handlers,
	}
	agentRuntime, err := deps.runtime(ctx, runtimeConfig)
	if err != nil {
		return nil, fmt.Errorf("create agent runtime: %w", err)
	}

	// Pro and Max model runtimes
	runtimes := map[string]modelRuntime{"lite": {runtime: agentRuntime, modelID: liteModelID}}
	for _, tier := range []string{"pro", "max"} {
		modelID := os.Getenv(map[string]string{"pro": "PRO_MODEL", "max": "MAX_MODEL"}[tier])
		if modelID == "" {
			continue
		}
		m, err := deps.model(ctx, modelID, tier)
		if err != nil {
			return nil, fmt.Errorf("initialize %s model: %w", tier, err)
		}
		runtimeConfig.ChatModel = m
		rt, err := deps.runtime(ctx, runtimeConfig)
		if err != nil {
			return nil, fmt.Errorf("create %s runtime: %w", tier, err)
		}
		runtimes[tier] = modelRuntime{runtime: rt, modelID: modelID}
	}
	service := &Service{
		runtimes:              runtimes,
		closeResources:        closeResources,
		tongjiClient:          tongjiClient,
		durableSessionStore:   postgresStore,
		ephemeralSessionStore: redisStore,
		turnLocker:            redisStore,
		taskPlanRepository:    taskPlanRepository,
		historyMessageLimit:   sessionConfig.HistoryMessageLimit,
	}
	initialized = true
	return service, nil
}

func defaultInitializationDeps() initializationDeps {
	return initializationDeps{
		instruction:    loadSystemInstruction,
		model:          modelprovider.NewFromEnv,
		knowledge:      knowledge.NewFromEnv,
		tongji:         tongjiapi.NewFromEnv,
		catalog:        agenticskills.Catalog,
		tavily:         tavily.NewFromEnv,
		webfetch:       webfetch.NewFromEnv,
		verifyChromium: webfetch.VerifyChromium,
		mcp:            mcpintegration.NewRemoteClientFromEnv,
		mcpTools:       mcpintegration.EinoTools,
		sandboxEnabled: sandbox.EnabledFromEnv,
		middleware:     sandbox.NewFileSystemMiddleware,
		sessionConfig:  sessionconfig.ConfigFromEnv,
		postgres:       sessionpostgres.NewPostgresStoreFromEnv,
		schema:         sessionpostgres.EnsurePostgresSchema,
		redis:          sessionredis.NewRedisEphemeralStoreFromEnv,
		taskplan: func(p *sessionpostgres.PostgresStore, r *sessionredis.RedisEphemeralStore) (taskplan.TaskPlanRepository, error) {
			return taskplan.NewTaskPlanRepository(p, r)
		},
		runtime:       func(ctx context.Context, cfg runtime.Config) (sessionRuntime, error) { return runtime.New(ctx, cfg) },
		closeMCP:      func(c *mcpclient.Client) error { return c.Close() },
		closePostgres: func(p *sessionpostgres.PostgresStore) { p.Close() },
		closeRedis:    func(r *sessionredis.RedisEphemeralStore) error { return r.Close() },
	}
}
