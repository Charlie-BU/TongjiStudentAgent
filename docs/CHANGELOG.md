## CHANGELOG - 2026-07-28 16:41 - 收敛远程 MCP 工具注册、请求级鉴权与失败结果边界

### 撰写时间

- 2026-07-28 16:41

### Base Commit

- 14660c3e1e3d71b0087d5ef868540f56c73398d5

### Compare Scope

- working_tree_only

### 背景与改动目标

- 远程 MCP 已成为个人数据能力的实际执行端，但 Tool 的发现、请求级 token 注入和上游失败结果此前集中在单个文件中。随着错误脱敏、allowlist 完整性和调用取消语义同时进入链路，原实现难以明确区分“业务可预期失败”和“应终止本次 Agent Run 的取消”。
- 这次把目标收敛为两点：第一，远程 Tool 只能由维护在应用层的 allowlist 注册；第二，MCP 返回的错误不能把上游原文带入模型上下文或 SSE 链路，同时仍需保留用户可理解的稳定状态。

### 改动概览

- 将 MCP 集成拆分为 `client.go`、`tools.go`、`request_scoped_tool.go` 和 `tool_result.go`。`client.go` 只负责环境配置、远程 Client 创建、启动和 Initialize；Tool 发现、请求包装及结果归一各自有独立职责。
- `internal/application/allowlist/tool` 改为私有 `mcpTools` 配合 `MCPTools()` 副本返回，并新增空值、空白名称和重复项校验。`chat.NewFromEnv` 只将这份 allowlist 传给 `mcpintegration.EinoTools`，远程能力缺项时继续在启动阶段失败。
- `requestScopedTool` 在本次调用 context 中读取 `AccessTokenFromContext`，缺失 token 时本地短路为稳定未授权结果；存在 token 时通过 `einoext.WithCustomHeaders` 仅向该次 MCP 调用附加 `X-Tongji-Access-Token`。取消和 deadline 错误继续向上返回，其余调用错误收敛为安全 JSON 结果。
- `ToolCallResultHandler` 会把 MCP 的 `IsError` 结果映射为 `unauthorized`、`upstream_timeout`、`upstream_unavailable` 或 `tool_execution_failed`，并丢弃上游原始消息。离线 MCP 测试补充了重复/空 allowlist、业务错误脱敏和传输超时场景。
- 聊天输入相关参数统一命名为 `query`。同时暂时停用把知识库内容直接拼入用户输入的路径，当前 Runtime 只接收原始 query，等待后续改为显式知识库 Tool。

### 关键链路解析（含上下游）

- 上游依赖：`handler.Chat` 与 `handler.ChatStream` 仍通过 `withChatAccessToken` 解析格式正确的 Bearer token 并写入 request context；无 token 或格式错误的 token 不会阻止 Agent 运行。`sandbox.EnabledFromEnv` 的读取和 Runtime middleware 组装路径保持不变。
- 当前改动：`chat.NewFromEnv` 读取 `toolallowlist.MCPTools()`，`EinoTools` 先校验 allowlist 并从远程 MCP 获取对应 schema，再把同步 Tool 包装为 `requestScopedTool`。实际 `tools/call` 时才读取 context token；MCP 业务错误在结果处理器中被替换为不含上游细节的稳定 JSON，网络错误则在包装器中按 timeout 与不可用状态归类。
- 下游影响：Deep Agent 只看见 allowlist 内的 Tool 和稳定工具结果，普通响应、日志和 SSE 事件不会携带上游错误正文或 token。取消仍作为错误中断 Agent Run，避免把用户主动取消误报为可继续消费的工具结果。

### 改动结果与业务影响

- Tool 注册从可变导出切片改为副本返回，调用方修改返回值不会污染后续服务初始化；空 allowlist、空名称和重复名称也会在远程发现前被拒绝。
- 校园服务侧的错误信息不再直接进入模型上下文。对于 token 缺失、token 无效、上游超时和通用执行失败，模型获得的是明确且有限的状态与中文提示，可据此向用户给出可理解的下一步。
- 已执行 `go test ./...`、`go test -race ./...`、`go vet ./...` 与 `git diff --cached --check`，均通过。测试使用本地回环 MCP 服务和 fake invokable Tool，不读取 `.env`，不调用真实模型、校园平台或知识库服务。

### 风险与待办

- 知识库直接注入虽然已停止，但 `NewFromEnv` 仍会按 `ARK_KNOWLEDGE_ENABLED` 初始化 `knowledgeClient`，README 也仍描述启用后的检索行为。因此知识库配置可能继续影响启动，却不再产生检索结果；在恢复为 Tool 调用或正式移除该能力前，需要同步清理初始化、配置说明和测试。
- `IsError` 被转换为普通 Tool 结果后，Runtime 会将这类调用作为 `tool.call.completed` 投影，而非 `tool.call.failed`。这适合让模型根据稳定状态继续回复；若前端需要区分远程业务失败，应在不暴露原始结果的前提下扩展事件状态。
- 当前仅覆盖同步 `InvokableTool`。后续若远程 MCP 引入流式或异步 Tool，需要补齐等价的请求级 header 注入、取消传播和错误归一机制。

### 建议 Commit Message（git-cz）

- `refactor(mcp): isolate tool auth and normalize failures`

## CHANGELOG - 2026-07-28 15:45 - 接入远程 MCP 并为个人数据工具透传请求级凭据

### 撰写时间

- 2026-07-28 15:45

### Base Commit

- f04040f91e99d0b1ae1e0a36c684df1526503786

### Compare Scope

- working_tree_only

### 背景与改动目标

- 原 Agent 只连接进程内 `get_current_time` MCP demo，无法验证与独立 `TongjiStudentMCPServer` 的部署协议，也不能把浏览器请求中已有的短期校园 access token 安全地交给个人数据工具。
- 本次目标是将启动依赖切换到远程 Streamable HTTP MCP，按应用 allowlist 发现可用 Tool，并让每次个人数据 Tool 调用仅从当前请求 context 读取凭据、按需注入远程请求；缺失 token 时不得触发远程调用。

### 改动概览

- 删除进程内 MCP Server、`get_current_time` demo 及其测试；新增 `internal/integration/mcp/client.go`，从 `MCP_SERVER_URL`、`MCP_TIMEOUT` 读取远程连接配置，完成 Client 创建、启动、Initialize、Tool Schema 发现和 allowlist 完整性校验。
- application allowlist 从单一包拆分为 `prompt`、`skill`、`tool` 子包；当前 Tool allowlist 固定为 `tongji.student.score`。聊天服务初始化时改为连接远程 MCP，并只将该 allowlist 转换为 Eino Tool。
- 为 Eino MCP Tool 增加请求级包装：仅在 `AccessTokenFromContext` 成功时附加 `X-Tongji-Access-Token`；缺失 token 时返回本地未授权提示而不访问 MCP。请求头按单次 `tool.Option` 传递，不写入 Client 全局状态或普通日志。
- README 与开发文档同步远程 MCP 必填环境变量、远程调用链、token 注入行为、缺失 token 的拒绝语义及远程服务的凭据保护责任。
- 新增离线 MCP 测试，覆盖配置解析、远程初始化失败、allowlist 缺项、无 token 不发请求，以及不同请求 token 的独立头透传。

### 关键链路解析（含上下游）

- 上游依赖：HTTP Handler 继续只把格式正确的 Bearer token 放入 request context；`chat.NewFromEnv` 在模型与知识库初始化后读取 MCP 配置、连接远程 Client，再传入 Tool allowlist。服务启动会在远程连接、Initialize 或允许工具发现失败时中止。
- 当前改动：`EinoTools` 先调用远程 `ListTools` 取得 allowlist 中的 schema，再将可同步调用的 Tool 包装为 `requestScopedTool`。运行时调用包装器时才读取本次 context 并使用 `einoext.WithCustomHeaders` 向该次 `tools/call` 请求添加校园 token。
- 下游影响：DeepAgent 不再获得本地时间 demo，而获得远程成绩 Tool；无 token 调用不会越过本地进程，有 token 调用由远程 MCP/同济开放平台继续验证有效期、用户绑定与 scope。远程 MCP 部署必须接受 Streamable HTTP 协议，并避免记录 `X-Tongji-Access-Token`。

### 改动结果与业务影响

- Agent 启动成功现在同时验证模型配置、远程 MCP 初始化和 allowlist 工具发现，减少运行到首次 Tool 调用才发现服务端能力不匹配的情况。
- 同一进程内的并发请求不共享 token：包装器按调用 context 创建请求头；测试验证连续调用分别到达测试 MCP 的对应 token，且 Tool 响应不回显凭据。
- 已执行 `go test ./...`、`go test -race ./...`、`go vet ./...`、`git diff --check` 与 `git diff --cached --check`，均通过。MCP 测试仅使用本机回环测试服务，不读取 `.env`、不访问真实校园平台或模型。

### 风险与待办

- 远程 MCP 成为启动前置依赖；网络不可达、协议不兼容或 allowlist 漂移都会阻止服务启动。生产部署应配置独立健康检查、连接超时和受控发布顺序。
- 当前本地仅检查 Bearer 格式并转发 token，不验证签名、有效期、用户身份或 scope；这些安全判定必须在可信远程 MCP 或同济开放平台落实，不能由 Agent 模型推断。
- 目前仅包装同步 Invokable Tool。若远程 MCP 后续提供流式或异步工具，需要扩展等价的请求级 header 注入和取消/错误传播测试，避免回退到无凭据或全局凭据路径。

### 建议 Commit Message（git-cz）

- `feat(mcp): add remote client and request-scoped token forwarding`

## CHANGELOG - 2026-07-28 12:05 - 为单轮 Agent 增加安全 SSE 运行事件与断连取消

### 撰写时间

- 2026-07-28 12:05

### Base Commit

- 6eef10764f5be419a9c3fac73d13d22ed9445163

### Compare Scope

- working_tree_only

### 背景与改动目标

- 原有 `POST /v1/agent/chat` 只能在单轮 Agent 完成后返回聚合文本，前端无法展示模型文本增量、知识库检索状态或工具执行进度；同时，直接将运行时内部消息暴露给 HTTP 响应会带来工具参数、工具结果、模型推理内容和 Bearer token 泄露风险。
- 本次在保持 JSON Chat 协议兼容的前提下，新增单轮 SSE 投影链路。目标是只向调用方发送经过裁剪的运行状态与最终回答文本增量，并在客户端断开导致 SSE 写入失败时取消本次 Agent 执行，避免无消费者的模型和工具调用继续占用资源。

### 改动概览

- 新增 `internal/agentic/event`，用 `run_id`、递增 `seq` 与 UTC `occurred_at` 标识单次运行事件；事件类型覆盖开始、状态、文本增量、工具开始/完成/失败和运行完成/失败。Emitter 只接收调用层准备好的展示数据，不承载凭据、原始工具结果或 reasoning content。
- `chat.Service`、`runtime.Runtime` 新增 `Stream` 链路：知识库阶段和模型阶段发送状态事件；Runtime 读取 Eino 流时仅发送 Assistant 文本与工具名称/耗时，并在工具结果流、Agent 事件或无文本结果失败时为未完成工具补发 `tool.call.failed`。
- 新增 `POST /v1/agent/chat/stream`。Handler 复用既有 Bearer context 与 JSON 请求校验，以 Hertz SSE writer 发送事件；普通 `/v1/agent/chat` 仍聚合并返回最终 JSON 文本。
- SSE 写入或事件序列化失败会取消本次专用 context，阻止后续事件写入，并只记录不包含请求内容的失败日志。README 与开发文档同步新接口、事件边界与当前无会话限制。
- 新增 Emitter、Runtime 工具失败、SSE 协议/非法请求和 SSE 写入失败取消上下文的离线 GoConvey 测试。

### 关键链路解析（含上下游）

- 上游依赖：Hertz 路由将 `/v1/agent/chat/stream` 分发到 `handler.ChatStream`；Handler 延续 `withChatAccessToken` 的可选 Bearer 透传和 `bindChatMessage` 的 JSON/空消息校验。SSE writer 负责设置 `text/event-stream`、`Cache-Control: no-cache` 与分块刷新。
- 当前改动：Handler 将请求 context 包装为可取消的 stream context，并把安全事件回调交给 `chat.Stream`。服务层用 Emitter 统一编号，随后将输入交给 Runtime；Runtime 消费 Eino Agent 事件并将工具生命周期、Assistant 可见文本投影为事件，不写出工具参数、工具结果和模型原始推理字段。
- 下游影响：JSON 调用方继续获得 `{"message":"..."}` 聚合结果；SSE 调用方可按 `run_id` 和 `seq` 消费单次运行过程。模型、知识库与 MCP 调用均收到同一个 stream context，写端断开后可沿既有 context 取消链路停止；尚未实现跨请求 Run 注册、事件重连、心跳或显式取消接口。

### 改动结果与业务影响

- 前端可在不保存会话历史的情况下展示 Agent 已开始、知识库检索、模型生成、工具执行和最终完成/失败状态，同时只消费设计为对外可见的数据字段。
- SSE 客户端断开或响应流不可写时，Handler 会取消本次运行并抑制后续写入，避免继续向失效连接发送事件；工具流异常也会以明确的工具失败事件收尾，不再只留下已开始状态。
- 已执行 `go test ./...`、`go test -race ./...`、`go test -race ./biz/handler`、`go vet ./...` 与 `git diff --check`，均通过。测试使用内存 SSE writer、fake Agent 与 Eino stream，不读取 `.env`、不调用真实模型、校园平台、知识库或 MCP。

### 风险与待办

- 当前 `run_id` 仅服务于 SSE 事件关联，没有进程级 Run 注册表；客户端主动取消、断线后的状态查询、重连续传和多实例协调仍需要引入受调用者归属保护的运行记录与存储。
- 取消依赖下游模型、知识库和 MCP 客户端遵守 context。第三方适配器若忽略取消信号，仍可能完成其已经发起的远程调用；后续应补充各外部适配器的超时与取消契约测试。
- SSE 仍没有心跳和代理空闲超时策略，长时间无事件的运行可能被中间层关闭；上线前应结合网关超时配置补充 keepalive 与可恢复的事件协议。

### 建议 Commit Message（git-cz）

- `feat(agent): add safe SSE run events and cancellation`

## CHANGELOG - 2026-07-28 01:49 - 收敛 Chat 请求元数据并为后续校园凭据透传预留上下文

### 撰写时间

- 2026-07-28 01:49

### Base Commit

- 4456fd1fd880e4d46270f01a242d4e41d73f771e

### Compare Scope

- working_tree_only

### 背景与改动目标

- Chat 入口此前会记录完整 Header、请求体和 Agent 回复。校园场景下，这会让浏览器 Bearer token、用户问题和未来的 Tool Result 进入普通日志；同时后续远程 MCP 需要使用浏览器会话中的短期校园 token，但当前 Agent 还没有实际的 token 验证与用户授权链路。
- 因此这次不把 token 格式检查包装成认证。目标是先删除普通日志中的敏感内容，建立请求关联 ID，并把格式正确的 Bearer token 作为可选的单请求上下文保存；没有 token 或格式错误时，Agent 仍按原路径调用。

### 改动概览

- 新增 `internal/platform/auth`：`ExtractBearerToken` 只解析标准 Bearer 格式，`WithAccessToken` 和 `AccessTokenFromContext` 使用私有 context key 保存与读取 token。未引入 token 持久化、模型消息注入或日志输出。
- `biz/handler.Chat` 改为可选调用 `withChatAccessToken`。合法 token 会传入 `chat.Chat` 的 context；缺失、Basic 或格式错误的 header 会保留原 context 并继续处理消息，不返回 401。
- `main.go` 的请求日志改为仅记录随机 `X-Request-ID`、方法、路径、状态码和耗时，不再记录 Header、Body 或响应内容；响应改用 `X-Enable-Stream`，并增加 `main_test.go` 固定 Request ID 与流式响应头契约。
- 删除 Chat 服务输出完整 Agent 回复的普通日志；运行配置的优雅退出变量从 `_BYTEFAAS_FUNC_TIMEOUT` 调整为 `FUNC_TIMEOUT`。README、开发文档和风险文档同步当前 token 边界与日志约束。
- 新增 Bearer 解析、请求上下文与 Handler 辅助函数测试，覆盖合法 token 写入及缺失、Basic、缺段和多段值被忽略的场景。

### 关键链路解析（含上下游）

- 上游依赖：浏览器仍通过同济 OAuth 回调取得短期 access token；`POST /v1/agent/chat` 可以选择性放入 `Authorization: Bearer <access_token>`。Hertz 的 `requestLoggingMiddleware` 在路由前生成 Request ID，随后 `streamHeaderMiddleware` 写入当前响应头。
- 当前改动：Handler 仅把通过 `ExtractBearerToken` 的 token 写入 context，再将该 context 交给 `chat.Chat`、知识库和 Runtime。普通日志从请求/响应全量序列化改为元数据记录，避免 token 和对话内容默认落盘。
- 下游影响：当前进程内 MCP demo 没有读取 access token，因此 Chat 的业务结果与无 token 客户端保持兼容。未来远程校园 MCP 接入时可从 `auth.AccessTokenFromContext(ctx)` 读取 token；在此之前，token 不构成身份、授权或访问控制依据。

### 改动结果与业务影响

- 调用方无需等待完整认证体系即可继续使用 Chat；携带合法 Bearer token 的请求则为后续 MCP 调用保留了不经过模型、响应和普通日志的传递通道。
- `X-Request-ID` 提供了不携带用户内容的排障关联标识，`X-Enable-Stream` 替代旧的 Bytefaas 专用响应头。改动同时缩小了 Agent 回复和 HTTP 请求内容的日志暴露面。
- 已执行 `go test ./...`、`go test -race ./biz/handler ./internal/platform/auth ./internal/application/chat .`、`go vet ./...` 与 `git diff --check HEAD`，均通过。测试不读取 `.env`、不调用真实模型或校园平台；全仓 `httptest` 用例在允许 loopback 的环境中运行。

### 风险与待办

- 当前 token 仅有格式解析，没有签名/有效期验证、用户绑定、scope 审核或远程 MCP 消费方。个人数据工具上线前必须在可信网关或同济开放平台完成这些授权步骤，不能依赖当前 context 值。
- `X-Bytefaas-Enable-Stream` 已被移除。若部署平台或旧客户端仍依赖该头，需要在上线前确认迁移策略，或显式保留兼容层。
- 日志缩减仅覆盖普通 HTTP 与 Chat 回复路径；未来若增加诊断采样、远程 MCP 或第三方 callback，应单独设计受限安全日志、字段脱敏和访问审计。

### 建议 Commit Message（git-cz）

- `feat(http): add request context and safe logging`

## CHANGELOG - 2026-07-28 00:54 - 迁移至 CozeLoop Trace 与 PromptHub 系统提示词

### 撰写时间

- 2026-07-28 00:54

### Base Commit

- 0be5bba6b1782f3d5033d2e850b278a5bcf8ec66

### Compare Scope

- working_tree_only

### 背景与改动目标

- 原来的 Fornax 集成依赖内部模块，虽然被隔离在 `internal/integration/fornax`，但仍让根 Go module 携带内部依赖，也无法承担开源部署下的 Prompt 管理需求。
- 这次将可选观测能力迁移到开源 CozeLoop，并把系统 Prompt 的获取固定在服务启动阶段。目标是在不改变 `/v1/agent/chat` 协议和默认关闭路径的前提下，形成“可选 Trace + PromptHub 配置 + Runtime Instruction”的清晰链路。

### 改动概览

- 删除 Fornax adapter 及其内部依赖，升级 `github.com/cloudwego/eino`，引入公开的 CozeLoop SDK、Eino callback 与 PromptHub 组件；`go.mod`、`go.sum` 因此收敛为公开依赖树。
- 新增 `internal/integration/cozeloop`：`COZELOOP_ENABLED` 默认关闭；开启时创建 SDK client、注册 Eino 全局 callback，并在关闭钩子中释放 client。启动预检要求 workspace ID、JWT OAuth client ID、public key ID 与 private key 全部存在，避免把缺失私钥延后为 SDK 的泛化认证失败。
- 新增 `internal/application/prompt/keys.go`，将 `prompt.tongjistudent.system_prompt` 作为集中管理的 PromptHub 标识。聊天服务在 CozeLoop 启用时拉取、格式化并提取 System 消息；`runtime.Config.Instruction` 再将其交给 `deep.New`。
- README 同步 CozeLoop 环境变量、PromptHub 行为和依赖前置条件，移除 Fornax 专用说明；现有聊天、Runtime 与 CozeLoop 测试同步覆盖新开关、未初始化 client、System 消息拼接以及缺失 JWT 私钥。

### 关键链路解析（含上下游）

- 上游依赖：`initializeClient` 仍先执行 `godotenv.Load`，随后初始化 CozeLoop，再调用 `chat.Init`。因此 `.env` 中的 `COZELOOP_*` 变量会在 PromptHub 拉取前就被读取；关闭开关时不会创建远端 client，也不会请求 PromptHub。
- 当前改动：`cozeloop.Init` 只在显式启用后注册 `cozeloopcallback.NewLoopHandler`。`chat.NewFromEnv` 调用 `loadSystemInstruction`，经 `FetchPrompt` 取得远端消息，并用 `MessageContent(..., schema.System)` 合并 System 文本；失败会中止服务初始化，而不会用不完整 Prompt 静默启动。
- 下游影响：`runtime.New` 现在把 `Config.Instruction` 转发给 Deep Agent，后续 `/v1/agent/chat` 的请求由同一 Runtime 消费该系统指令。HTTP Handler、知识库注入、MCP 与 Sandbox 的外部协议没有改动；关闭时仍由既有 shutdown hooks 依序关闭 Agent 与 CozeLoop client。

### 改动结果与业务影响

- 默认运行路径继续不启用外部 Trace 或 PromptHub。显式启用后，模型调用会被 Eino 全局 callback 观测，并使用 PromptHub 中的系统提示词，而不需要把 Prompt 固化在二进制代码中。
- 这次额外处理了一个配置边界：README 已声明 JWT 私钥是必需项，但初版预检遗漏它。现在缺少私钥会在创建 SDK client 前返回包含变量名的配置错误，并有离线回归测试。
- 已执行 `go test ./...`、`go test -race ./internal/integration/cozeloop ./internal/application/chat`、`go vet ./...` 与 `git diff --check HEAD`，均通过。首次在受限沙箱运行全仓测试时，既有 `httptest` 用例因端口绑定限制失败；在允许本地 loopback 的环境复跑后通过。

### 风险与待办

- CozeLoop 开启后，服务启动依赖远端 PromptHub 可用性与 `prompt.tongjistudent.system_prompt` 的内容结构。当前选择失败即停止启动，以避免在未知系统指令下服务；生产环境仍需用独立 workspace 验证权限、Prompt 版本与可用性。
- 当前仅接受完整 JWT OAuth 配置，未提供 `COZELOOP_API_TOKEN` 的测试用替代分支。若本地调试需要 API Token，应先明确安全边界，再实现二选一的配置校验与文档。
- `callbacks.AppendGlobalHandlers` 和 CozeLoop client 都是进程级资源，当前初始化路径假设服务进程只初始化一次。未来若引入热重载或同进程多应用实例，需要补充 handler 去重与 client 生命周期管理。

### 建议 Commit Message（git-cz）

- `feat(observability): migrate Fornax integration to CozeLoop`

## CHANGELOG - 2026-07-23 01:01 - 接入同济开放平台浏览器 OAuth 授权链路

### 撰写时间

- 2026-07-23 01:01

### Base Commit

- 63f26c7fad980c9dd0e2babc4c6ce674999bdbf9

### Compare Scope

- working_tree_only

### 背景与改动目标

- 这次改动的起点是校园数据能力还没有浏览器侧的受控授权入口。前端回调页位于 `app.tongji.edu.cn`，而 Go 服务负责保存开放平台客户端密钥并交换授权码，因此不能把 `client_secret`、refresh token 或完整的 OAuth 请求体交给浏览器和通用请求日志。
- 我们最终采用授权码模式：服务先生成带有效期的签名 `state` 并跳转到同济开放平台，回调页面再把 `code` 和 `state` 交回 Go 服务换取短期 access token。目标是先打通当前浏览器会话的最小链路，同时将敏感字段的暴露面控制在服务端。

### 改动概览

- 新增 `internal/integration/tongjiapi.Client`：从 `TONGJI_OPEN_PLATFORM_*` 环境变量读取客户端配置，构造授权地址，以 `HMAC-SHA256` 签名并校验十分钟有效的 `state`，使用表单方式交换授权码，并提供受保护开放平台数据接口的基础 GET 调用。
- 新增 `GET /v1/tongji/oauth/authorize`、`POST /v1/tongji/oauth/token` 与对应 OPTIONS 路由。Handler 仅把 access token、类型、过期时间和 scope 返回给登记的 callback 页面，不向浏览器返回 refresh token 或 ID token。
- `main.go` 对 `/v1/tongji/oauth/` 使用精简日志，只记录方法、路径、状态码和耗时，避免授权码、state、客户端凭据和令牌被写入请求/响应日志；README 同步补充 `.env` 变量、浏览器回调方式和前端存储边界。
- 新增并迁移 OAuth 测试至 GoConvey。Handler 测试覆盖授权重定向、配置错误、非法 JSON、空字段、伪造 state、上游失败、CORS 预检和 refresh token 脱敏；客户端测试使用本地 `httptest` 或自定义 `RoundTripper` 覆盖成功请求、无效输入与上游 HTTP/业务错误。

### 关键链路解析（含上下游）

- 上游依赖：`initializeClient` 既有的 `godotenv.Load` 负责载入 `.env`；`tongjiapi.NewFromEnv` 在 Handler 收到请求时读取 `TONGJI_OPEN_PLATFORM_CLIENT_ID`、`CLIENT_SECRET`、`REDIRECT_URI` 与 `STATE_SECRET`。同济开放平台是授权页、令牌端点和数据 API 的外部提供者，但单元测试不访问其真实地址。
- 当前改动：浏览器访问 `/authorize` 后，`TongjiAuthorize` 调用 `CreateState` 和 `AuthorizationURL` 发出 302；callback 页面 POST 到 `/token` 后，`TongjiExchangeToken` 先校验 JSON、字段和签名 state，再由 `ExchangeAuthorizationCode` 发送 `application/x-www-form-urlencoded` 请求。响应转换为 `tongjiBearerToken`，因此上游返回的 refresh token 不会越过服务端边界。
- 下游影响：Hertz 路由新增 OAuth 入口，但既有 `/v1/ping` 和 `/v1/agent/chat` 的协议没有变化。浏览器跨域读取仅对 `https://app.tongji.edu.cn` 返回 CORS 响应头；后续学生数据 MCP 可复用 `tongjiapi.Client` 的 access-token 数据请求能力，但当前还没有 token 持久化或用户身份绑定。

### 改动结果与业务影响

- 当前可以从浏览器发起同济统一认证，并在 callback 页面获得短期 Bearer access token 用于本次会话；前端文档明确要求只在内存保存，并在过期后重新授权。
- 已执行 `go test -count=1 ./biz/handler ./internal/integration/tongjiapi`、`go test -race -count=1 ./biz/handler ./internal/integration/tongjiapi`、`go vet ./biz/handler ./internal/integration/tongjiapi` 和 `git diff --check HEAD`，均通过。测试会验证令牌交换的请求方法、表单字段和响应脱敏，不需要真实开放平台凭据。
- 这次改动没有改变 Agent 聊天调用链，也没有把 refresh token 传给 Agent、模型上下文或通用日志。

### 风险与待办

- 当前 `state` 只验证签名和时效，尚未与发起授权的浏览器会话绑定，也没有一次性消费 nonce；它不能完整承担 OAuth CSRF/响应关联保护。后续需要引入受控的短期 state 存储或安全会话 Cookie，并同步调整 callback 页的跨域凭据策略。
- 目前 access token 只返回给 callback 页，没有受信任用户身份、refresh token 安全存储或 token 续期机制。接入学生数据 MCP 前，需要明确用户绑定、最小 scope、失效与撤销策略。
- 生产环境仍需登记实际 Go 服务回调配置，并用独立测试账号完成授权平台的契约验证；本次测试只覆盖离线 HTTP 契约，未验证真实平台字段或重定向行为。

### 建议 Commit Message（git-cz）

- `feat(oauth): add Tongji Open Platform authorization flow`

## CHANGELOG - 2026-07-19 18:42 - 建立单元测试规范并迁移至 GoConvey 场景

### 撰写时间

- 2026-07-19 18:42

### Base Commit

- 33fe7a3142d5ba94db4a70b33c77bc18bfa502ce

### Compare Scope

- working_tree_only

### 背景与改动目标

- 这次改动的起点是测试要求分散在代码习惯里：聊天服务、Runtime、知识库、MCP、Sandbox 的测试虽然已有覆盖，但场景组织、断言风格、外部依赖隔离和质量门禁并不统一。随着 `internal` 分层落地，这种差异会让后续新增能力难以判断最小验证范围。
- 因此本次先把单元测试要求沉淀为可执行规范，再将现有测试迁移到 GoConvey。目标不是增加真实外部调用，而是让离线、确定性测试能明确表达正常、配置、错误和安全开关分支。

### 改动概览

- 新增 `docs/UTSpec.md` 与 `.codex/rules/unit-testing.md`，约定 GoConvey 场景组织、mock 工具选择、环境变量和包级状态清理、离线边界、竞态检查及各模块最低覆盖范围。
- 更新 `commit-quality-reviewer`：审阅 Go 代码、测试或依赖改动时，要求读取 UTSpec、覆盖受影响测试包，并执行 `go test`、必要的 `-race` 和 `go vet`。
- 为 Go module 加入公开的 `github.com/smartystreets/goconvey`；`github.com/volcengine/volc-sdk-golang` 改为直接依赖，以反映知识库生产代码的实际导入关系。
- 将 Runtime、chat Service、Fornax、知识库、MCP 和 Sandbox 的现有测试重构为嵌套 `Convey`/`So` 场景，同时保留环境变量恢复、MCP Client 关闭和自定义 HTTP Transport 等隔离措施。

### 关键链路解析（含上下游）

- 上游依赖：测试入口仍是 Go 标准 `testing`，GoConvey 只提供场景和断言。知识库测试继续使用自定义 `RoundTripper`，MCP 测试继续使用进程内 demo Client，因此不会访问真实 Ark、知识库、校园平台或远程 MCP。
- 当前改动：`docs/UTSpec.md` 规定了 `internal/application/chat`、`internal/agentic/runtime` 及各 integration adapter 的测试边界；对应测试以中文业务场景表达配置错误、未初始化、开关解析、请求签名和工具调用结果。
- 下游影响：新的审阅 skill 将测试验证纳入固定输出格式。后续修改 Go 代码或依赖时，开发与审阅都需要按同一套质量门禁执行，避免只依赖编译通过。

### 改动结果与业务影响

- 单测运行保持离线、确定性，不读取 `.env` 的真实凭据，不执行本机 Shell，也不调用真实模型或知识库。
- 已执行 `go test -count=1 ./...`、`go test -race -count=1 ./...`、`go vet ./...` 和 `git diff --check HEAD`，均通过。竞态检测期间出现 macOS 链接器警告，但未导致测试或竞态检查失败。
- 当前新增规范已覆盖核心模块的最低场景清单；实际覆盖率阈值尚未接入 CI 门禁，后续需要补充 coverage 报告与阈值校验。

### 风险与待办

- GoConvey 增加了测试依赖及其间接依赖，后续升级需继续验证 Go 版本兼容性和离线缓存可用性。
- 测试当前覆盖到现有的错误与适配路径，但不替代真实环境的契约验证；包含 Ark、知识库和校园平台的端到端验证仍应使用独立测试账号与脱敏数据。
- 知识库不可信内容注入风险仍按 `WL-20260719-001` 临时豁免至 2026-08-19，测试规范未改变该安全边界。

### 建议 Commit Message（git-cz）

- `test(core): standardize unit tests with GoConvey`

## CHANGELOG - 2026-07-19 18:12 - 将单轮聊天运行链路迁移至 internal 分层

### 撰写时间

- 2026-07-19 18:12

### Base Commit

- 9804870e82620c0a3206fba9eebb3c6d7ccea1fe

### Compare Scope

- working_tree_only

### 背景与改动目标

- 原有 `agent`、顶层 `integration`、`mcpserver` 与 `pkg/logging` 将应用编排、Eino Runtime 和具体基础设施放在同一层。随着知识库、MCP、Fornax 和本地 Sandbox 逐步加入，入口依赖越来越难以说明，因此这次先完成一轮不改变 HTTP 协议的路径归位。
- 目标是让 `biz/handler` 只负责 HTTP 适配，`internal/application/chat` 负责聊天服务装配，`internal/agentic/runtime` 只依赖 Eino 抽象，具体 Ark、MCP、知识库、Sandbox、配置和日志实现统一进入 `internal/integration` 与 `internal/platform`。

### 改动概览

- 新增 `internal/application/chat.Service`，承接模型、知识库、MCP、Sandbox 和 Runtime 的初始化、单轮聊天及 MCP 资源关闭；`init.go` 与 `/v1/agent/chat` 改为调用该应用服务。
- 新增 `internal/agentic/runtime`，将 Deep Agent 的构造和事件迭代收敛到 `Runtime.New`、`Runtime.Chat`，以 `model.BaseChatModel`、`tool.BaseTool` 和 middleware 作为输入契约。
- 将 Ark 模型、知识库、Fornax、本地 MCP 与 Sandbox adapter 移入 `internal/integration`，将服务端口、优雅退出和日志移入 `internal/platform`；删除原路径上的重复实现。
- 更新 README、开发设计和风险文档，使目录、Sandbox 装配开关及当前日志风险与实际代码保持一致。
- 将提示词注入临时豁免 `WL-20260719-001` 的匹配路径同步到 `internal/application/chat/service.go`；新增 Runtime 与聊天服务的最小回归测试，覆盖依赖缺失、未初始化、无知识库直通等分支。

### 关键链路解析（含上下游）

- 上游依赖：`godotenv.Load` 继续在 `initializeClient` 中加载 `.env`；Fornax 初始化后，`chat.Init` 组装 Ark ChatModel、可选知识库客户端、本地 MCP Client、Sandbox middleware 和 Runtime。
- 当前改动：`biz/handler.Chat` 仍校验请求体并返回相同 JSON 协议，但由 `chat.Chat` 驱动服务；`Service.Chat` 先注入知识库参考资料，再委托 `Runtime.Chat` 读取最终 Assistant 文本。
- 下游影响：Hertz 路由、单轮非流式响应和进程关闭钩子的外部行为保持不变。MCP Client 的关闭责任从 `agent.CloseMcpClient` 转为 `chat.Close`，由既有 shutdown hook 调用。

### 改动结果与业务影响

- 当前运行时边界更明确：`internal/agentic/runtime` 不直接依赖 Ark、MCP、知识库或本地 Sandbox；协议与基础设施替换集中在 integration/platform 层。
- 已执行 `go test ./...`、`go vet ./...` 与 `git diff --check HEAD`。迁移后的 MCP、知识库、Fornax、Sandbox 测试仍可通过，新增 Runtime 与 chat Service 的最小错误路径测试也已覆盖。
- HTTP handler 到真实 Ark 模型、知识库和 MCP 的完整端到端调用仍依赖外部凭据，未在本地测试中执行。

### 风险与待办

- 知识库内容仍作为非可信文本拼入模型输入；该项按 `WL-20260719-001` 临时豁免至 2026-08-19，后续应改为隔离的数据或工具结果通道。
- `SANDBOX_ENABLED=true` 仍会提供宿主机文件与 Shell 能力，公开部署必须保持关闭；完整请求和响应日志也仍可能包含敏感数据，详见 `docs/RISKS.md`。
- 本次迁移中的 `internal/` 文件当前仍未跟踪，提交时必须与旧路径删除和入口改动一并纳入，避免出现只删除旧包而遗漏新实现的构建失败。

### 建议 Commit Message（git-cz）

- `refactor(agent): move chat runtime and adapters into internal layers`

## CHANGELOG - 2026-07-19 17:52 - 将本地 Sandbox 改为显式开关装配

### 撰写时间

- 2026-07-19 17:52

### Base Commit

- 0741f446b482fc43d577d305faf28c67af0838b2

### Compare Scope

- working_tree_only

### 背景与改动目标

- 本地文件系统和 Shell middleware 仍有调试价值，但默认装配会把宿主机文件操作与命令执行能力交给 Agent。校园服务的公开运行环境不需要这些能力，因此这次把“是否装配”从代码注释和人工修改，收敛成 `.env` 中可审计的 `SANDBOX_ENABLED` 开关。
- 同时清理已经不再适配当前运行方式的 Hertz 配置和部署脚本，并修正文档中仍引用旧 `bootstrap.sh` 的描述，避免本地开发与部署预期继续漂移。

### 改动概览

- 新增 `integration/sandbox` adapter。`EnabledFromEnv` 解析 `SANDBOX_ENABLED`：变量缺省或为 `false` 时关闭，合法的 `true`/`1` 时开启，非法值会在启动阶段返回配置错误。
- `agent.InitDeepAgentAndMcpClient` 改为先读取开关，再决定是否调用 `sandbox.NewFileSystemMiddleware` 并加入 Deep Agent 的 `Handlers`。本地 Backend、`filesystem.New` 和 `StreamingShell` 的实现从 `agent` 包迁出。
- `README.md` 说明开关默认关闭、开启后的本机权限范围和公开部署限制，同时将已删除的 `script/bootstrap.sh` 替换为 `local_run.sh` 指引。
- `docs/DEVELOPMENT.md` 更新迁移期 adapter 边界与装配条件；删除未被当前运行链路消费的 `conf/hertz.config.yaml`、`script/bootstrap.sh` 和 `unpack.sh`。
- 为 `SANDBOX_ENABLED` 增加表驱动测试，覆盖缺省、显式关闭、`true`、`1` 和非法配置。

### 关键链路解析（含上下游）

- 上游依赖：`initializeClient` 通过 `godotenv.Load` 载入项目根目录的 `.env`；因此 `SANDBOX_ENABLED` 可由本地配置或进程环境统一提供，无需新增命令行参数。
- 当前改动：Agent 初始化在模型、知识库和 MCP 工具准备完成后读取开关。关闭时传入空的 `Handlers`；开启时才由 `integration/sandbox` 创建本地 Backend 和文件系统 middleware。
- 下游影响：`biz/handler.Chat` 与 HTTP 响应结构没有变化。默认情况下，模型不再获得文件操作和 Shell 工具；仅受控本地开发显式开启时才恢复这部分能力。

### 改动结果与业务影响

- 默认运行路径不再装配宿主机文件系统与命令执行能力，公开部署的最小权限边界更清晰。
- 本地需要文件处理调试时，可在 `.env` 写入 `SANDBOX_ENABLED=true`，无需改动 Agent 初始化代码。
- 已执行 `go test ./...`、`go vet ./...` 与 `git diff --check HEAD`。开关解析有单元测试；尚未针对真实 Agent 调用链做开启 sandbox 后的端到端工具执行验证。

### 风险与待办

- `SANDBOX_ENABLED=true` 仍使用本机 Backend 和 `StreamingShell`，因此只能在受控开发环境启用。部署配置需要显式保持为 `false`，并在发布流程中检查该变量。
- 删除旧部署脚本前已检查仓库内引用；仓库外部若仍有 CI/CD 或运行平台依赖这些脚本，需要同步迁移到当前构建和启动方式。
- 当前 `integration/sandbox` 及其测试仍是未跟踪文件，提交时必须与 `agent/agent.go` 一并纳入，否则启用开关的构建将失败。

### 建议 Commit Message（git-cz）

- `feat(agent): gate local sandbox with environment config`

## CHANGELOG - 2026-07-19 14:21 - 为主 Agent 接入可选 Ark 知识库检索

### 撰写时间

- 2026-07-19 14:21

### Base Commit

- b0521ee2466d8975faf2f591e6084af032eea6a6

### Compare Scope

- working_tree_only

### 背景与改动目标

- 这次改动的目标是让学生问答在保留现有 Deep Agent、MCP 和本地文件工具链的前提下，可选地引用 Ark 知识库。知识库关闭时必须维持原有对话路径；开启后，启动阶段应尽早发现凭据或资源标识缺失，避免请求进来后才暴露配置错误。
- 审阅中识别到“将不可信资料拼入提示词”的风险。该项已按当前决策登记为临时豁免，白名单会在 2026-08-19 到期，届时需要复核隔离方案，不能将其视为已解决问题。

### 改动概览

- 新增 `integration/ark/knowledge`：从 `ARK_KNOWLEDGE_*` 环境变量创建客户端，使用 AK/SK 签名调用 `/api/knowledge/collection/search_knowledge`，并把命中切片格式化为参考资料。
- `agent.InitDeepAgentAndMcpClient` 初始化知识库客户端；`agent.Chat` 在创建 runner 后检索当前用户问题，并仅在有命中时将资料和原问题一起交给 Deep Agent。
- `README.md` 与 `local_run.sh` 补充知识库开关、凭据、集合/资源 ID 和 limit 配置说明及启动前校验；Agent 显示名称同步为 `Tongji Student Agent`。
- 新增 `NewFromEnv`、`Search` 的表驱动测试。测试 HTTP 客户端会验证请求方法、地址、签名相关 headers 和 JSON 请求体，并覆盖传输、HTTP、业务码和 JSON 解析失败路径。
- 更新 `.codex` 的审阅白名单与声明注释规则；删除已迁移的 DeepWiki 注入规则，并调整 `commit-update-writer` 的 commit subject 约束。

### 关键链路解析（含上下游）

- 上游依赖：`godotenv.Load` 负责加载 `.env`，`local_run.sh` 在本地运行时导出同一组变量；`InitDeepAgentAndMcpClient` 是服务启动时创建 Agent 和 MCP 客户端的唯一入口。
- 当前改动：`knowledge.NewFromEnv` 在开关关闭时返回 `nil`，因此 `withKnowledgeContext` 会原样返回用户消息。开关开启时，`Client.Search` 使用调用请求的 `context` 和 10 秒 HTTP 超时访问知识库；`FormatContext` 只保留命中的标题、内容和 FAQ 原问题。
- 下游影响：`biz/handler.Chat` 仍通过 `agent.Chat` 返回相同的 HTTP 响应结构。知识库命中会增加一次网络 I/O 和模型输入长度；检索失败目前会使本次对话返回错误，而不是降级为无知识库回答。

### 改动结果与业务影响

- 当前支持按 `ARK_KNOWLEDGE_ENABLED` 显式启用。关闭时不创建知识库客户端，对既有聊天调用链没有额外请求。
- 启用后可通过集合名或资源 ID 检索，返回内容以“非可信参考资料”标记注入同一次模型调用，没有新增独立模型调用链。
- 已执行 `go test ./...`，并验证了客户端的配置解析和请求构造。尚未使用真实 Ark 凭据完成端到端检索验证。

### 风险与待办

- 当前资料仍以文本形式与用户消息共同传给模型。即使有非可信资料约束，恶意切片仍可能影响模型行为；该风险已登记为 `WL-20260719-001`，到期前应改为隔离的工具结果/数据通道，并限制工具决策受不可信资料影响。
- 知识库服务不可用会直接使 `agent.Chat` 失败。后续需要根据产品预期决定是否改为记录错误后降级回答，并补充相应测试。
- `integration/ark/knowledge`、测试和 `local_run.sh` 目前仍是未跟踪文件，提交时必须与 `agent/agent.go` 一并纳入，否则干净检出环境无法编译或使用本地启动脚本。

### 建议 Commit Message（git-cz）

- `feat(agent): add optional Ark knowledge retrieval with request tests`
