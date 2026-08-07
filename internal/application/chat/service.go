// Package chat 提供当前聊天应用服务。
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/runtime"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	sessionconfig "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/config"
	sessionpostgres "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/postgres"
	sessionredis "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/redis"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	promptallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/prompt"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/arkmodel"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/cozeloop"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	mcpintegration "github.com/Charlie-BU/TongjiStudent/internal/integration/mcp"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/sandbox"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

var defaultService *Service

// studentInfoLoader 按当前请求凭据获取学生基础信息。
type studentInfoLoader func(ctx context.Context, accessToken string) (*tongjiapi.StudentInfo, error)

// sessionRuntime 描述会话执行链路所需的最小运行时能力。
type sessionRuntime interface {
	StreamWithHistoryAndMessages(ctx context.Context, query, studentInfo string, history []agenticsession.Message, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error)
}

// Service 组装聊天、会话 Runtime 与外部适配器。
type Service struct {
	runtime               sessionRuntime                    // Agent Runtime
	mcpClient             *mcpclient.Client                 // MCP Client
	knowledgeClient       *knowledge.Client                 // 知识库 Client
	studentInfoLoader     studentInfoLoader                 // 个人信息加载器
	durableSessionStore   agenticsession.Store              // 认证会话存储
	ephemeralSessionStore agenticsession.EphemeralStore     // 匿名会话存储
	turnLocker            agenticsession.TurnLocker         // 会话执行锁
	postgresSessionStore  *sessionpostgres.PostgresStore    // PostgreSQL 资源
	redisSessionStore     *sessionredis.RedisEphemeralStore // Redis 资源
	taskPlanRepository    taskplan.TaskPlanRepository       // 当前会话任务计划
	historyMessageLimit   int                               // 上下文历史消息上限
}

// Init 从环境变量初始化默认聊天服务。
func Init(ctx context.Context) error {
	service, err := NewFromEnv(ctx)
	if err != nil {
		return err
	}
	defaultService = service
	return nil
}

// NewFromEnv 从环境变量构造聊天服务。
func NewFromEnv(ctx context.Context) (*Service, error) {
	instruction, err := loadSystemInstruction(ctx)
	if err != nil {
		return nil, err
	}

	// 模型相关
	chatModel, err := arkmodel.NewFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize chat model: %w", err)
	}

	// 知识库相关
	knowledgeClient, err := knowledge.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("initialize knowledge client: %w", err)
	}

	// 系统提示词相关
	// 工具相关
	mcpClient, err := mcpintegration.NewRemoteClientFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize remote mcp client: %w", err)
	}
	MCPTools, err := mcpintegration.EinoTools(ctx, mcpClient, toolallowlist.MCPTools()...)
	if err != nil {
		_ = mcpClient.Close()
		return nil, fmt.Errorf("convert mcp tools: %w", err)
	}
	// skill 相关
	skillCatalog, err := agenticskills.Catalog()
	if err != nil {
		return nil, fmt.Errorf("build skill catalog: %w", err)
	}

	handlers := []adk.ChatModelAgentMiddleware{}
	// 沙箱相关
	sandboxEnabled, err := sandbox.EnabledFromEnv()
	if err != nil {
		_ = mcpClient.Close()
		return nil, fmt.Errorf("read sandbox configuration: %w", err)
	}
	if sandboxEnabled {
		filesystemMiddleware, err := sandbox.NewFileSystemMiddleware(ctx)
		if err != nil {
			_ = mcpClient.Close()
			return nil, fmt.Errorf("create filesystem middleware: %w", err)
		}
		handlers = append(handlers, filesystemMiddleware)
	}

	// session 持久化相关
	sessionConfig, err := sessionconfig.ConfigFromEnv()
	if err != nil {
		_ = mcpClient.Close()
		return nil, fmt.Errorf("read session configuration: %w", err)
	}
	postgresStore, err := sessionpostgres.NewPostgresStoreFromEnv(ctx)
	if err != nil {
		_ = mcpClient.Close()
		return nil, fmt.Errorf("initialize PostgreSQL session store: %w", err)
	}
	if err := sessionpostgres.EnsurePostgresSchema(ctx, postgresStore); err != nil {
		postgresStore.Close()
		_ = mcpClient.Close()
		return nil, fmt.Errorf("initialize PostgreSQL session schema: %w", err)
	}
	redisStore, err := sessionredis.NewRedisEphemeralStoreFromEnv(ctx, sessionConfig.AnonymousTTL, sessionConfig.AnonymousMessageLimit)
	if err != nil {
		postgresStore.Close()
		_ = mcpClient.Close()
		return nil, fmt.Errorf("initialize Redis session store: %w", err)
	}
	// task plan 相关
	taskPlanRepository, err := taskplan.NewTaskPlanRepository(postgresStore, redisStore)
	if err != nil {
		_ = redisStore.Close()
		postgresStore.Close()
		_ = mcpClient.Close()
		return nil, fmt.Errorf("initialize task plan repository: %w", err)
	}
	tools := append(systemtools.Tools(
		systemtools.WithTaskPlanRepository(taskPlanRepository),
		systemtools.WithKnowledgeClient(knowledgeClient),
	), MCPTools...)
	rt, err := runtime.New(ctx, runtime.Config{
		Name:          "Tongji Student Agent",
		Description:   "Campus assistant that answers questions using approved Tongji services.",
		Instruction:   instruction,
		SkillCatalog:  skillCatalog,
		ChatModel:     chatModel,
		Tools:         tools,
		MaxIterations: 12,
		Handlers:      handlers,
	})
	if err != nil {
		_ = redisStore.Close()
		postgresStore.Close()
		_ = mcpClient.Close()
		return nil, err
	}

	return &Service{
		runtime:               rt,
		mcpClient:             mcpClient,
		knowledgeClient:       knowledgeClient,
		studentInfoLoader:     loadStudentInfo,
		durableSessionStore:   postgresStore,
		ephemeralSessionStore: redisStore,
		turnLocker:            redisStore,
		postgresSessionStore:  postgresStore,
		redisSessionStore:     redisStore,
		taskPlanRepository:    taskPlanRepository,
		historyMessageLimit:   sessionConfig.HistoryMessageLimit,
	}, nil
}

// loadSystemInstruction 从 Cozeloop PromptHub 加载 system prompt。
func loadSystemInstruction(ctx context.Context) (string, error) {
	if !cozeloop.Enabled() {
		return "", nil
	}

	messages, err := cozeloop.FetchPrompt(ctx, promptallowlist.TongjiStudentSystemPrompt, "", nil)
	if err != nil {
		return "", fmt.Errorf("load system prompt: %w", err)
	}

	instruction, err := cozeloop.MessageContent(messages, schema.System)
	if err != nil {
		return "", fmt.Errorf("system prompt %q: %w", promptallowlist.TongjiStudentSystemPrompt, err)
	}
	return instruction, nil
}

// CreateSession 为已认证或匿名请求创建对应生命周期的会话。
func CreateSession(ctx context.Context) (agenticsession.Session, error) {
	if defaultService == nil {
		return agenticsession.Session{}, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.CreateSession(ctx)
}

// StreamSession 提交会话消息并以 SSE 事件返回本轮执行过程。
func StreamSession(ctx context.Context, sessionID, query string, send func(agentevent.Event)) (string, error) {
	if defaultService == nil {
		return "", fmt.Errorf("chat service is not initialized")
	}
	return defaultService.StreamSession(ctx, sessionID, query, send)
}

// ListSessionMessages 读取当前请求有权访问的会话历史。
func ListSessionMessages(ctx context.Context, sessionID string, limit int) ([]agenticsession.Message, error) {
	if defaultService == nil {
		return nil, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.ListSessionMessages(ctx, sessionID, limit)
}

// GetSessionTaskPlan 读取当前请求有权访问的会话任务计划。
func GetSessionTaskPlan(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error) {
	if defaultService == nil {
		return nil, fmt.Errorf("chat service is not initialized")
	}
	return defaultService.GetSessionTaskPlan(ctx, sessionID)
}

// Close 释放默认聊天服务持有的资源。
func Close() error {
	if defaultService == nil {
		return nil
	}
	return defaultService.Close()
}

// CreateSession 为当前请求的身份状态创建会话。
func (s *Service) CreateSession(ctx context.Context) (agenticsession.Session, error) {
	if s == nil {
		return agenticsession.Session{}, fmt.Errorf("chat service is not initialized")
	}
	// userId 存在时，创建持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return agenticsession.Session{}, fmt.Errorf("durable session store is not initialized")
		}
		return s.durableSessionStore.Create(ctx, ownerUserID)
	}
	// userId 不存在时，创建临时会话
	if s.ephemeralSessionStore == nil {
		return agenticsession.Session{}, fmt.Errorf("ephemeral session store is not initialized")
	}
	return s.ephemeralSessionStore.Create(ctx)
}

// StreamSession 将当前用户消息、历史和最终回答写入同一会话。
func (s *Service) StreamSession(ctx context.Context, sessionID, query string, send func(agentevent.Event)) (string, error) {
	runID := agentevent.NewRunID()
	releaseTurn, err := s.acquireSessionTurn(ctx, sessionID)
	if err != nil {
		s.emitSessionFailure(runID, send, err)
		return "", err
	}
	defer releaseTurn()
	// 创建 taskPlanScope
	scope, err := s.taskPlanScope(ctx, sessionID)
	if err != nil {
		s.emitSessionFailure(runID, send, err)
		return "", err
	}
	// 绑定 taskPlanScope 到 context
	runCtx := taskplan.WithTaskPlanScope(ctx, scope)
	activeTaskPlan, err := s.activeTaskPlan(runCtx)
	if err != nil {
		s.emitSessionFailure(runID, send, err)
		return "", err
	}
	runCtx = taskplan.WithActiveTaskPlan(runCtx, activeTaskPlan)

	history, appendUser, err := s.sessionTurnOperations(runCtx, sessionID, query, runID)
	if err != nil {
		s.emitSessionFailure(runID, send, err)
		return "", err
	}
	return s.stream(runCtx, runID, query, history, func() error {
		_, err := appendUser() // 在 Agent 执行前追加用户消息到 session
		return err
	}, func(ctx context.Context, message *schema.Message) error {
		return s.appendAgentMessage(ctx, sessionID, runID, message)
	}, send)
}

// GetSessionTaskPlan 读取当前请求有权访问的会话任务计划。
func (s *Service) GetSessionTaskPlan(ctx context.Context, sessionID string) (*taskplan.TaskPlan, error) {
	scope, err := s.taskPlanScope(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.activeTaskPlan(taskplan.WithTaskPlanScope(ctx, scope))
}

func (s *Service) activeTaskPlan(ctx context.Context) (*taskplan.TaskPlan, error) {
	if s == nil || s.taskPlanRepository == nil {
		return nil, fmt.Errorf("task plan repository is not initialized")
	}
	return s.taskPlanRepository.GetTaskPlan(ctx)
}

// taskPlanScope 在 Runtime 启动前再次读取已授权 session，并据此创建 TaskPlanScope。
func (s *Service) taskPlanScope(ctx context.Context, sessionID string) (taskplan.TaskPlanScope, error) {
	if s == nil {
		return taskplan.TaskPlanScope{}, fmt.Errorf("chat service is not initialized")
	}
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return taskplan.TaskPlanScope{}, fmt.Errorf("durable session store is not initialized")
		}
		session, err := s.durableSessionStore.Get(ctx, sessionID, ownerUserID)
		if err != nil {
			return taskplan.TaskPlanScope{}, err
		}
		return taskplan.NewTaskPlanScope(session)
	}
	if s.ephemeralSessionStore == nil {
		return taskplan.TaskPlanScope{}, fmt.Errorf("ephemeral session store is not initialized")
	}
	session, err := s.ephemeralSessionStore.Get(ctx, sessionID)
	if err != nil {
		return taskplan.TaskPlanScope{}, err
	}
	return taskplan.NewTaskPlanScope(session)
}

// acquireSessionTurn 为会话获取执行锁，确保并发安全。
func (s *Service) acquireSessionTurn(ctx context.Context, sessionID string) (agenticsession.TurnRelease, error) {
	if s == nil || s.turnLocker == nil {
		return nil, fmt.Errorf("session turn locker is not initialized")
	}
	return s.turnLocker.AcquireTurn(ctx, sessionID)
}

// emitSessionFailure 发送会话失败事件。
func (s *Service) emitSessionFailure(runID string, send func(agentevent.Event), err error) {
	emitter := agentevent.NewEmitter(runID, send)
	emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
	code, message := "session_unavailable", "会话不存在或暂时不可用"
	if errors.Is(err, agenticsession.ErrTurnInProgress) {
		code, message = "turn_in_progress", "该会话正在处理中，请稍后重试"
	}
	emitter.Emit(agentevent.RunFailed, agentevent.RunFailedData{Code: code, Message: message})
}

// ListSessionMessages 读取当前请求可访问的会话消息。
func (s *Service) ListSessionMessages(ctx context.Context, sessionID string, limit int) ([]agenticsession.Message, error) {
	if s == nil {
		return nil, fmt.Errorf("chat service is not initialized")
	}
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return nil, fmt.Errorf("durable session store is not initialized")
		}
		return s.durableSessionStore.ListMessages(ctx, sessionID, ownerUserID, limit)
	}
	if s.ephemeralSessionStore == nil {
		return nil, fmt.Errorf("ephemeral session store is not initialized")
	}
	return s.ephemeralSessionStore.ListMessages(ctx, sessionID, limit)
}

// sessionTurnOperations 为本轮选择存储、读取历史并构造用户消息追加操作。
// 返回会话历史、追加用户消息函数和错误。
func (s *Service) sessionTurnOperations(ctx context.Context, sessionID, query, runID string) ([]agenticsession.Message, func() (agenticsession.AppendResult, error), error) {
	if s == nil {
		return nil, nil, fmt.Errorf("chat service is not initialized")
	}
	// userId 存在时，选择持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		if s.durableSessionStore == nil {
			return nil, nil, fmt.Errorf("durable session store is not initialized")
		}
		history, err := s.durableSessionStore.ListMessages(ctx, sessionID, ownerUserID, s.historyLimit())
		if err != nil {
			return nil, nil, err
		}
		return history,
			func() (agenticsession.AppendResult, error) {
				return s.durableSessionStore.Append(ctx, sessionID, ownerUserID, agenticsession.NewMessage{RunID: runID, Role: agenticsession.MessageRoleUser, Content: query})
			}, nil
	}
	// userId 不存在时，选择临时会话
	if s.ephemeralSessionStore == nil {
		return nil, nil, fmt.Errorf("ephemeral session store is not initialized")
	}
	history, err := s.ephemeralSessionStore.ListMessages(ctx, sessionID, s.historyLimit())
	if err != nil {
		return nil, nil, err
	}
	return history,
		func() (agenticsession.AppendResult, error) {
			return s.ephemeralSessionStore.Append(ctx, sessionID, agenticsession.NewMessage{RunID: runID, Role: agenticsession.MessageRoleUser, Content: query})
		}, nil
}

// appendAgentMessage 追加 Agent 输出消息到会话。
// 参数：上下文、sessionID、本轮 runID、Agent 消息
// 返回：错误信息
func (s *Service) appendAgentMessage(ctx context.Context, sessionID, runID string, message *schema.Message) error {
	input, err := agenticsession.NewMessageFromSchema(message)
	if err != nil {
		return err
	}
	input.RunID = runID
	input.ResponseID, _ = ark.GetResponseID(message)
	input.ResponseCacheExpiresAt, _ = ark.GetCacheExpiration(message)
	// userId 存在时，选择持久化会话
	if ownerUserID, ok := platformauth.UserIDFromContext(ctx); ok {
		_, err = s.durableSessionStore.Append(ctx, sessionID, ownerUserID, input)
		return err
	}
	// userId 不存在时，选择临时会话
	_, err = s.ephemeralSessionStore.Append(ctx, sessionID, input)
	return err
}

// historyLimit 返回有效的上下文历史消息数量。
func (s *Service) historyLimit() int {
	if s.historyMessageLimit > 0 {
		return s.historyMessageLimit
	}
	return 20
}

// stream 执行一次模型调用，并通过 record 持久化每条 Agent 输出消息。
// 参数：上下文、本轮 runID、用户 query、历史消息、模型调用前回调、记录回调、事件发送函数
// 返回：模型回答、错误信息
func (s *Service) stream(ctx context.Context, runID, query string, history []agenticsession.Message, beforeModel func() error, record func(context.Context, *schema.Message) error, send func(agentevent.Event)) (string, error) {
	emitter := agentevent.NewEmitter(runID, send)
	if s == nil || s.runtime == nil {
		emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
		emitter.Emit(agentevent.RunFailed, agentevent.RunFailedData{Code: "agent_unavailable", Message: "Agent 服务暂不可用"})
		return "", fmt.Errorf("chat service is not initialized")
	}
	startedAt := time.Now()
	emitter.Emit(agentevent.RunStarted, agentevent.RunStartedData{Message: "Agent 已开始处理请求"})
	emitter.Emit(agentevent.AgentStatus, agentevent.AgentStatusData{Phase: "context", Message: "正在准备回答上下文"})
	studentInfo, err := s.loadFormattedStudentInfo(ctx)
	if err != nil {
		emitter.Emit(agentevent.RunFailed, agentevent.RunFailedData{Code: "student_info_unavailable", Message: "学生基础信息暂时不可用，请稍后重试"})
		return "", err
	}
	// Agent 执行前
	if beforeModel != nil {
		if err := beforeModel(); err != nil {
			code, message := "session_write_failed", "会话消息暂时无法保存，请稍后重试"
			if errors.Is(err, agenticsession.ErrTurnInProgress) {
				code, message = "turn_in_progress", "该消息正在处理中，请勿重复提交"
			}
			emitter.Emit(agentevent.RunFailed, agentevent.RunFailedData{Code: code, Message: message})
			return "", err
		}
	}

	// TODO：使用 tool 调用知识库检索工具，不要直接作为 input
	// input, err := s.withKnowledgeContextWithEmitter(ctx, query, emitter)
	// if err != nil {
	// 	emitter.Emit(agentevent.RunFailed, map[string]string{"code": "knowledge_search_failed", "message": "知识库检索暂时不可用"})
	// 	return "", err
	// }

	emitter.Emit(agentevent.AgentStatus, agentevent.AgentStatusData{Phase: "model", Message: "正在生成回答"})
	emitRuntimeEvent := func(event agentevent.Event) {
		emitter.Emit(event.Type, event.Data)
	}
	response, err := s.runtime.StreamWithHistoryAndMessages(ctx, query, studentInfo, history, emitRuntimeEvent, record)
	if err != nil {
		emitter.Emit(agentevent.RunFailed, agentevent.RunFailedData{Code: "agent_execution_failed", Message: "Agent 执行失败"})
		return "", err
	}
	emitter.Emit(agentevent.RunCompleted, agentevent.RunCompletedData{DurationMS: time.Since(startedAt).Milliseconds()})
	return response, nil
}

// loadFormattedStudentInfo 仅在请求上下文携带 access token 时读取学生基础信息。
func (s *Service) loadFormattedStudentInfo(ctx context.Context) (string, error) {
	accessToken, ok := platformauth.AccessTokenFromContext(ctx)
	if !ok || s.studentInfoLoader == nil {
		return "", nil
	}
	studentInfo, err := s.studentInfoLoader(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return tongjiapi.FormatStudentInfo(studentInfo), nil
}

// loadStudentInfo 通过同济开放平台获取当前授权学生的基础信息。
func loadStudentInfo(ctx context.Context, accessToken string) (*tongjiapi.StudentInfo, error) {
	client, err := tongjiapi.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create Tongji Open Platform client: %w", err)
	}
	studentInfo, err := client.GetStudentInfo(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("get Tongji student info: %w", err)
	}
	return studentInfo, nil
}

// Close 释放聊天服务持有的外部资源。
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	var closeErr error
	if s.redisSessionStore != nil {
		closeErr = errors.Join(closeErr, s.redisSessionStore.Close())
	}
	if s.postgresSessionStore != nil {
		s.postgresSessionStore.Close()
	}
	if s.mcpClient != nil {
		closeErr = errors.Join(closeErr, s.mcpClient.Close())
	}
	return closeErr
}
