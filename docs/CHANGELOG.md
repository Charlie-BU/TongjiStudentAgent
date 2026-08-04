## CHANGELOG - 2026-08-05 02:26 - 记录并回放 Agent 工具调用与推理轨迹

### 撰写时间

- 2026-08-05 02:26

### Base Commit

- ca79459eb67f07a734c665ae2581e9fb14f91105

### Compare Scope

- working_tree_only

### 背景与改动目标

- 已持久化的多轮会话此前只保存 user 消息与最终 assistant 文本。Agent 一旦在某轮调用工具，后续轮次恢复历史时既看不到 assistant 发起的 `ToolCalls`，也看不到对应的 tool result；模型无法重新建立「这条工具结果属于哪个调用」的关联，工具驱动的任务在跨请求续聊时会退化为不完整上下文。
- 产品同时需要在 SSE 中展示模型 reasoning、工具入参和工具结果，用于让前端呈现 Agent 的实际执行过程。这与原先“安全事件不暴露 Eino 内部结构”的约束相冲突。本次不将其伪装为默认安全行为，而是在审阅白名单中明确登记为 `WL-20260805-002`：该暴露由产品确认并接受，白名单于 2026-09-05 到期，届时必须重新审查数据范围与前端访问控制。
- 本次目标是在不改变现有会话归属、Redis/PostgreSQL 存储选择和 Tool allowlist 的前提下，将一轮 Agent 处理过的 canonical `schema.Message` 完整记录下来，并在下一轮以相同角色语义回放；同时让实时 SSE 与历史消息都能表达 reasoning、工具调用及工具结果。

### 改动概览

- `internal/agentic/event` 新增 `assistant.reasoning` 事件与 `AssistantReasoningData`；`tool.started` 新增 `arguments`，`tool.completed` 新增 `result`。事件包注释同步改为公共 SSE 协议，并明确数据可包含 reasoning、工具参数和结果，但不得放入 Bearer token、数据库连接串或服务端凭据。
- `runtime.Runtime` 在原有 `StreamWithHistory` 之上新增 `StreamWithHistoryAndMessages`。它在处理每个 `*schema.Message` 时调用记录回调：assistant 的 `ReasoningContent` 投影为 `assistant.reasoning`，tool call 的函数参数写入 `tool.started.arguments`，工具输出写入 `tool.completed.result`。旧方法继续委托新方法，保持现有调用方兼容。
- 会话 canonical 消息扩展 `tool` role、`ToolCalls`、`ToolCallID`、`ToolName` 与 `ReasoningContent`。`NewMessageFromSchema` 现在可保留 assistant tool call/reasoning 和 tool result；校验规则允许携带工具调用或 reasoning 的无正文 assistant 消息，并要求 tool 消息必须带 `tool_call_id`。
- `ContextAssembler` 恢复历史时不再只拼接 user/assistant 文本：它会按 sequence 回放 assistant 的 tool calls 与 reasoning，再回放关联的 tool 消息。下一次 `buildInputMessagesWithHistory` 因而能向 Agent 还原完整的「assistant 调用工具 -> tool 返回 -> assistant 继续回答」链路。
- PostgreSQL 的 `agent_session_messages` 增加 `tool_calls JSONB`、`tool_call_id`、`tool_name`、`reasoning_content`，迁移同时放宽 role 约束以接受 `tool`。Redis 消息 JSON 与 Lua append 参数同步携带这些字段；两种存储的 `Append`/`ListMessages` 都保留完整结构化消息。
- `chat.Service.StreamSession` 识别支持记录回调的 runtime，并将每条处理过的 schema 消息交给 `appendAgentMessage` 写入 durable 或 ephemeral store；启用该路径时不再在结束阶段重复追加 final assistant 文本，避免同一轮最终回复出现两份。
- `PostgresStore.pool` 收敛为只含 `Close`、`Exec`、`Query`、`QueryRow`、`BeginTx` 的内部接口，生产仍使用 `pgxpool.Pool`，测试则通过新增的 `pgxmock/v4` 精确验证建表迁移、事务写入和读取反序列化。运行时、上下文、handler、PostgreSQL 测试一并补齐工具轨迹断言。
- `README.md` 与 `docs/DEVELOPMENT.md` 更新为当前协议：前端会接收 reasoning、工具入参与工具结果，必须按会话归属处理；文档同样列出不应传输的 Bearer token、数据库连接串和服务端凭据。

### 关键链路解析（含上下游）

- 上游输入仍由 `biz/handler` 的会话 SSE 接口进入 `chat.Service.StreamSession`。服务先依据用户身份选择 PostgreSQL durable store 或 Redis ephemeral store，并在既有 turn lock 保护下读取 canonical 历史、写入本轮 user 消息；身份校验、会话归属和工具 allowlist 并未因本次改动放宽。
- 当前执行链变为 `chat.Service.StreamSession` -> `Runtime.StreamWithHistoryAndMessages` -> `Runtime.readMessage`。Runtime 一边把 assistant reasoning、tool started、tool completed 等事件投影到 SSE，一边把原始 `schema.Message` 交给记录回调；服务层将其映射为 `session.Message` 后追加到与该会话匹配的持久化或临时存储。最终 assistant 消息也由同一回调保存，避免旧的 after-model 分支再写一次。
- 持久化与恢复链为 `PostgresStore`/`RedisEphemeralStore` -> `ListMessages` -> `ContextAssembler` -> `buildInputMessagesWithHistory`。消息中的 `ToolCalls`、`ToolCallID`、`ToolName` 和 `ReasoningContent` 均按 sequence 读取并映射回 Eino schema，所以下游 Agent 看到的是具备调用关联的历史，而不是脱离来源的纯文本摘要。
- PostgreSQL 迁移先创建包含新字段的新表定义，再对已部署表执行 role check 替换和 `ADD COLUMN IF NOT EXISTS`；写入时将 `ToolCalls` 序列化到 JSONB，读取时反序列化。Redis 仍以 Lua 保证追加、序号分配、窗口裁剪和 TTL 刷新的原子性，只是消息 payload 扩展为同一套字段。
- SSE 下游从“只看最终回答和工具状态”变为可消费原始 reasoning、函数 arguments 与工具 result。字段属于显式产品协议而非服务端脱敏视图；前端、网关日志和任何事件转发消费者必须以当前会话的 owner/capability 为边界，不得把这些 payload 当作可公开广播的数据。

### 改动结果与业务影响

- 工具型对话现在可跨请求继续：下一轮模型能同时获得发起调用的 assistant 消息、对应的 tool result 以及后续 assistant 输出，不会因历史缺少 `tool_call_id` 关联而丢失执行上下文。
- 前端可在运行中呈现模型 reasoning、实际工具参数和工具结果；历史接口也会返回可表达同一轨迹的 canonical 消息字段。`Message` 的 JSON tag 统一为 lower snake case，handler 测试已按新输出更新。
- 已执行并通过 `go test ./...`、`go test -race ./internal/agentic/runtime ./internal/agentic/session ./internal/application/chat ./biz/handler`、`go vet ./...` 与 `git diff --check`。新增 PostgreSQL 测试覆盖 schema SQL、assistant 工具调用/reasoning 写入以及 tool result 读回后的顺序恢复。

### 风险、边界与后续建议

- `assistant.reasoning`、工具 arguments 与 result 是原始 Agent 轨迹，可能含用户输入、工具返回的敏感业务数据或第三方内容。本次由 `WL-20260805-002` 明确豁免，不代表数据天然安全；白名单到期前应复核前端会话授权、SSE/日志采集脱敏、持久化数据分级与删除策略。
- 当前测试使用 `pgxmock` 验证 SQL 调用契约和消息编解码，未在真实 PostgreSQL 实例执行迁移。应在 CI 或预发布环境补充真实 PG migration/round-trip 集成测试，特别验证已存在 `agent_session_messages` 表的约束替换与列补齐。
- 原始 tool payload 会增加 PostgreSQL/Redis 存储量及后续模型上下文长度；当前仅受会话消息窗口限制，尚无对 reasoning/arguments/results 的摘要、截断或单条大小上限。需要结合实际工具输出量设定容量和保留策略。
- 记录回调在模型流式过程中失败会使本轮运行失败，但此前已经记录的部分轨迹可能保留在会话中。若产品要求严格的整轮原子可见性，后续应引入 turn 状态或提交标记，并在历史读取时过滤未完成轮次。

### 建议提交信息

- `feat(session): persist agent tool traces`

## CHANGELOG - 2026-08-05 00:35 - 将 Agent 调用链切换为持久化多轮会话

### 撰写时间

- 2026-08-05 00:35

### Base Commit

- acb6bd12b3c9b3365703661c5f3a0593c6a0249a

### Compare Scope

- working_tree_only

### 背景与改动目标

- 上一次改动只完成了 `Session`、canonical 消息和 `Runtime.StreamWithHistory` 的领域边界，HTTP 调用仍是无状态的 `/v1/agent/chat` 与 `/v1/agent/chat/stream`。这意味着历史消息无法跨请求保存，已认证用户的会话归属、匿名用户的自动过期以及同一会话的并发执行都没有真正进入生产链路。
- 一开始可以继续沿用进程内 `InMemoryStore`，用它快速把 API 接起来；但它在进程重启、Pod 扩缩容和多实例路由下都不能恢复同一段对话，也无法成为身份会话的唯一事实源。因此本次将认证与匿名两类生命周期拆开：前者落 PostgreSQL，后者放带 TTL 的 Redis；模型仍只消费服务端读取、校验后的 canonical 历史。
- 目标不是为旧接口附加一个可选 `session_id`，而是明确一条新的会话协议：先创建会话，再提交消息，最后按归属读取历史。为避免多请求同时写入造成消息顺序混乱，本次也把跨实例的单会话执行锁一并接入。

### 改动概览

- 对外路由由单轮 `POST /v1/agent/chat`、`POST /v1/agent/chat/stream` 切换为三个会话接口：`POST /v1/sessions` 创建会话，`POST /v1/sessions/:session_id/messages` 以 SSE 执行并保存一轮消息，`GET /v1/sessions/:session_id/messages?limit=...` 读取 canonical 历史。SSE `Event` 新增 `session_id` 字段，使同一客户端可将运行事件与会话关联。
- `chat.Service` 新增 `CreateSession`、`StreamSession`、`ListSessionMessages`，并从原先直接调用 Runtime 的单轮 `Stream` 改为“取历史 -> 写用户消息 -> 调模型 -> 写最终助手消息”的编排。运行成功前后分别执行 append；模型失败时不写助手回复，写入失败时向 SSE 发送稳定的 `session_write_failed` 终态。
- 新增 `PostgresStore`：从 `POSTGRES_DSN` 建立并 Ping `pgxpool`，启动时通过 `EnsurePostgresSchema` 创建 `agent_sessions`、`agent_session_messages`、唯一顺序约束与查询索引。认证会话的 `Get`、`Append`、`ListMessages` 始终带 `owner_user_id`；追加消息在事务中对会话行 `FOR UPDATE`，再分配连续 `sequence` 并更新 `last_active_at`。
- 新增 `RedisEphemeralStore`：从 `REDIS_URL` 连接 Redis，使用 `SESSION_ANONYMOUS_TTL`（默认 24h）与 `SESSION_ANONYMOUS_MAX_MESSAGES`（默认 20）管理匿名会话。Lua 脚本原子地校验 meta key、分配序号、写入消息、裁剪最近消息并刷新两个 key 的 TTL；`miniredis` 测试覆盖了上限裁剪和过期行为。
- 新增 Redis `TurnLocker`。`AcquireTurn` 以 `SET NX PX` 获得 30 秒锁，持锁期间每 10 秒以 token 校验方式续租，释放时只删除自己持有的锁。`chat.Service.StreamSession` 在读写历史前持锁；锁冲突会以 `turn_in_progress` 结束本轮 SSE，避免两个 Run 并发插入同一会话。
- 删除此前只用于领域阶段的 `memory.go` / `memory_test.go`，同时移除 `Message.ClientTurnID` 与 `NewMessage.ClientTurnID`。当前生产链路尚未实现客户端重试幂等键，文档与测试不再宣称该能力已经存在。
- `runtime.Runtime` 收敛到 `StreamWithHistory`，应用层以最小 `sessionRuntime` 接口注入该能力。Runtime、动态提醒、学生资料、Skill catalog、历史和当前请求仍由 `buildInputMessagesWithHistory` 统一装配，既不允许客户端提交任意历史，也不改变 Tool allowlist、RunState 或安全事件裁剪边界。
- `README.md`、`docs/DEVELOPMENT.md` 同步了新 API、`POSTGRES_DSN`、`REDIS_URL` 与三个会话容量配置；`go.mod` / `go.sum` 新增 `pgx/v5`、`go-redis/v9` 及仅测试使用的 `miniredis`。
- 补充配置解析、Redis 会话、PostgreSQL 输入/DDL、服务编排和 HTTP handler 测试。handler 覆盖创建成功与 503、历史读取和 404、SSE 中的 `session_id` 投影、缺失 `session_id` 的 400；同时在审阅白名单中登记了旧 Agent API 的有意下线，`WL-20260805-001` 于 2026-09-05 到期，届时必须复核迁移是否完成。

### 关键链路解析（含上下游）

- 上游依赖：`biz/handler` 仍通过 `withChatAccessToken` 从 `Authorization` 中提取 Bearer token。该上下文会沿用已有的 `UserIDFromContext`：存在用户 ID 时，服务选择 PostgreSQL durable store；缺失用户 ID 时，服务只创建和访问 Redis ephemeral store。服务启动新增两个必填基础设施依赖，因此模型、MCP 初始化成功不再足以代表服务可用，PostgreSQL schema 与 Redis Ping 也必须成功。
- 当前改动：创建接口只调用 `chat.CreateSession` 并返回 `session_id`、`persistence`。提交接口先校验 JSON 消息和 path `session_id`，再将 HTTP context 派生为可取消的 SSE context；`StreamSession` 取得 Redis 锁、按当前身份读取有限窗口历史、写入用户消息，随后调用 `Runtime.StreamWithHistory`。模型返回非空最终回答后才追加 assistant 消息，最后由事件发送器输出 `run.completed`。每个写给 SSE 的事件由 handler 补入当前 `session_id`，但不暴露 token、工具参数、原始工具响应或模型 reasoning。
- 持久化路径：认证请求的 `ListMessages` 和每次 `Append` 都将 `session_id + owner_user_id` 作为查询条件；`Append` 还在同一事务中锁住 session row，使 `MAX(sequence) + 1` 不会被同一会话的并发写入竞争。匿名请求不包含 owner，而是把随机 `anon_` 标识作为短期 capability；Redis meta/messages key 同时续期，消息列表按时间正序返回最近窗口。
- 下游影响：DeepAgent 现在首次获得前轮 user/assistant 的结构化消息，而不是只收到当前 XML `interaction_request`；静态 Skill、远程 MCP 工具和学生资料读取仍从同一 request context 获取必要状态。调用方必须先保存 `POST /v1/sessions` 的结果，再使用新路径提交或读取；原先直接调用旧 `/v1/agent/chat*` 的客户端不会自动兼容。

### 改动结果与业务影响

- 已认证用户的会话可以跨进程通过 PostgreSQL 恢复，且读取与写入都受 `owner_user_id` 限制；这为多实例部署提供了最小的所有权边界。匿名对话不落 PostgreSQL，默认会在 24 小时后自动过期，并且每个会话只保存最近 20 条消息，避免无身份调用无限积累。
- 模型上下文现在由服务端规范化的历史驱动。历史窗口由 `SESSION_HISTORY_MAX_MESSAGES` 控制，默认 20；当前轮 user 消息会在模型开始前持久化，assistant 最终文本只在生成成功后持久化。因此失败 Run 会留下用户输入但不会伪造一条成功助手回复，这既保留了可追踪的用户动作，也要求前端能处理“最后一条 user 消息尚无回复”的状态。
- 同会话的并发提交不再同时进入模型运行。锁冲突不会静默覆盖历史，而是以 SSE `run.failed` / `turn_in_progress` 明确返回。数据库侧的事务锁和 Redis 侧的分布式执行锁分工不同：前者保障 durable 消息序号，后者保障完整 Run 生命周期。
- 这次将旧单轮接口直接替换为会话接口，是有意的破坏性 API 变更。审阅时已识别该风险，产品确认客户端会随发布迁移，因此通过 `WL-20260805-001` 进行短期豁免；该条目不是永久兼容策略。
- 已验证：`go test ./...`、`go test -race ./internal/agentic/session ./internal/application/chat ./biz/handler`、`go vet ./...`、`git diff --check` 均通过。测试使用 fake Runtime、内存 SSE writer 和 `miniredis`，不访问真实模型、校园平台、共享 Redis、PostgreSQL 或宿主机 Shell。

### 风险与待办

- `POSTGRES_DSN` 与 `REDIS_URL` 现在都是启动必填项。任一基础设施不可达都会令 Agent 服务初始化失败；当前没有 feature flag、降级到无状态聊天或延迟连接策略。发布前需要确认部署环境的网络、凭据、数据库权限以及 schema 创建权限已经就绪。
- 匿名会话以随机 `session_id` 作为访问能力，不额外绑定 Cookie、设备或匿名身份；拿到该 ID 的调用方可以继续读取和提交这段匿名会话。当前随机 ID 足以避免枚举，但前端、日志、浏览器存储和链接传播都不应泄漏它。若匿名历史将承载敏感内容，后续应增加匿名会话绑定或显式的访问令牌设计。
- Redis 锁的续租在 Redis 临时不可用时会停止；若模型运行超过剩余 TTL，另一个请求理论上可重新获得锁。当前实现仍保留 PostgreSQL append 的顺序安全，但不能保证整个模型 Run 绝对单活跃。后续应补充续租失败的观测、Run 超时上限与失锁后的 fail-closed 策略。
- PostgreSQL 生产操作目前只有输入与 DDL 单测，没有独立临时 PostgreSQL 的 Create/Get/Append/所有权/事务并发集成测试；Redis 的 Lua 和 TTL 行为已经由 `miniredis` 覆盖，但也应在 CI 或预发布环境增加真实 Redis/PG 契约验证。
- 当前 `Message` 对外 JSON 仍使用 Go 默认字段名（例如 `Content`），而创建会话和 SSE 已使用 snake_case 协议字段。文档只描述了 canonical 历史，不应据此假定其 JSON 字段风格；在客户端接入前应明确 history response DTO 与字段兼容策略。
- `docs/DEVELOPMENT.md` 中有关未来 `client_turn_id` 幂等、CAS 和回滚开关的规划已调整，但本次没有实现请求重试幂等、断线重连、取消、HITL Resume、历史摘要或分页 cursor。长会话仍依赖固定条数窗口，不能保证 token 长度上界。

### 建议 Commit Message（git-cz）

- `feat(session): persist multi-turn chat sessions`

## CHANGELOG - 2026-08-03 17:32 - 建立 Agent 多轮会话领域基础与历史上下文装配

### 撰写时间

- 2026-08-03 17:32

### Base Commit

- a56895b17ded28dee06739d5b4a81964a5f86c19

### Compare Scope

- working_tree_only

### 背景与改动目标

- Agent 当前仍按单轮请求运行，但后续多轮能力需要先有稳定的会话归属、消息顺序、幂等写入和模型上下文边界。直接在聊天服务中拼接历史会把存储语义和模型输入格式耦合在一起，也难以独立验证并发与重放行为。
- 因此这次先提交领域层基础设施：定义不依赖 HTTP、数据库或 Agent 实现的会话契约，并让 Runtime 能接收已经规范化的历史消息。聊天入口的会话创建、读取和写入不在本次范围内。

### 改动概览

- 新增 `internal/agentic/session`：定义 `Session`、`Message`、`Store`、消息角色、持久化类型及输入校验错误；`InMemoryStore` 提供本地开发和单元测试使用的互斥访问、消息递增序号、用户消息 `client_turn_id` 幂等与最近消息读取能力。
- 新增 `ContextAssembler`，按“动态提醒 → canonical 历史 → 当前用户请求”的固定顺序转换为 Eino 消息；历史仅接受用户与助手角色，并校验内容和严格递增序号。
- Runtime 新增 `StreamWithHistory`，由 `buildInputMessagesWithHistory` 统一装配提醒、学生资料、Skill catalog、历史和当前 XML 请求。保留 `Stream` 与 `StreamWithStudentInfo` 的既有单轮兼容行为。
- 补充会话存储、上下文装配和运行时历史顺序测试；恢复 `trustedStudentInfoReminder` 对资料中标签/指令文本的 XML 转义回归测试。

### 关键链路解析（含上下游）

- 上游依赖：调用方需要先完成认证和会话归属校验，再从 `Store.ListMessages` 读取同一用户拥有的 canonical 消息。当前聊天服务仍调用 `StreamWithStudentInfo`，因此继续传入空历史，原有请求协议和单轮语义不变。
- 当前改动：应用层未来可把经过归属校验的历史传给 `Runtime.StreamWithHistory`；Runtime 将其交给 `ContextAssembler`，再交由 Deep Agent 执行。动态提醒和本轮 query 均保持用户消息角色，历史中的助手回复则保留助手角色，避免把多轮记录拼接成不具备角色语义的文本。
- 下游影响：后续接入持久化 Store、会话 API 或聊天服务时可以复用同一领域契约与装配器，无需改变模型运行主循环、Skill allowlist、MCP Tool 或 SSE 事件协议。由于本次未接入调用入口，不会立即启用多轮对话。

### 改动结果与业务影响

- 现在已经具备可独立测试的会话最小模型：用户消息重试可通过 `client_turn_id` 返回已保存记录，并发追加仍产生连续的 `Sequence`；模型输入只接受经过角色和顺序校验的历史。
- 保持现有 `Stream`/`StreamWithHistory` API 的行为不变，现有聊天继续作为无状态单轮运行。这个取舍是本次仅提交领域层基础设施的明确边界，而不是遗漏的生产接入。
- 已执行 `go test ./...`、`go test -race ./internal/agentic/runtime ./internal/agentic/session`、`go vet ./...` 与 `git diff --check`，均通过；本次恢复测试后再次执行 `go test ./internal/agentic/runtime`，通过。

### 风险与待办

- `InMemoryStore` 仅适用于本地开发和测试，进程重启后不会保留消息；虽然领域类型包含 `PersistenceDurable`，真正跨进程恢复仍需要后续接入持久化 Store。
- 生产接入时必须以当前授权用户 ID 做会话所有权校验，并在模型调用前读取有限长度历史、在成功生成后追加最终助手文本；不能直接接受客户端传入的任意历史消息。
- 当前测试覆盖存储并发、幂等、角色/顺序校验、历史插入位置和资料 XML 边界；后续接入聊天服务后应补充会话 API、取消、失败写入策略和完整多轮链路测试。

### 建议 Commit Message（git-cz）

- `feat(agent): add session domain foundation`

## CHANGELOG - 2026-07-31 01:06 - 为授权请求上下文补充校园用户 ID

### 撰写时间

- 2026-07-31 01:06

### Base Commit

- a513c6303ef8c3179616f4e488284a3d63d730bd

### Compare Scope

- working_tree_only

### 背景与改动目标

- 请求 context 原先仅保存 Bearer access token，后续需要以校园账户的稳定 `user_id` 支持会话归属和用户身份相关能力。本次在保留既有 token 传递语义的基础上，补充用户 ID 的获取与读取入口。

### 改动概览

- 新增同济开放平台用户基础信息适配器：以 Bearer token 调用 `/v2/rt/user/all_info`，解析 `data.list` 并返回首个用户的 `userId`。
- `platformauth.WithAccessToken` 在写入非空 token 后同步解析用户 ID；解析成功时写入同一 request context，新增 `UserIDFromContext` 供下游读取。解析失败仍保留 token，不改变现有聊天调用的降级行为。
- 补充客户端和 context 单元测试，并将运行时资料输入测试同步为 `user-profile-data/user-info` 数据边界。
- 在风险登记和审阅白名单中记录同步身份查询的临时 HIGH 风险豁免，豁免有效至 2026-08-30。

### 关键链路解析（含上下游）

- 上游依赖：HTTP Agent Handler 从 Authorization Header 提取 token，并在 JSON 请求体校验前调用 `WithAccessToken`；用户基础信息请求依赖 Tongji Open Platform 的环境配置和调用凭据。
- 当前改动：context setter 先规范化并保存 access token，再调用用户基础信息接口；仅在获得非空 `userId` 时追加该值。上游失败或数据不完整时不会阻断原有 token 流程。
- 下游影响：既有聊天、MCP 和 token 读取链路保持兼容；后续会话归属或授权能力可从 context 读取用户 ID。当前生产代码尚未消费该字段。

### 改动结果与业务影响

- 已授权请求可携带与校园账户对应的稳定用户 ID，为后续按用户维度的能力提供基础，且不会改变无 token 请求或上游失败时的聊天可用性。
- 本次为每个携带 token 的调用方增加一次用户基础信息查询；该查询是 `WithAccessToken` 的隐式网络副作用。

### 风险与待办

- 同步查询发生在请求体校验之前，可能使无效请求也访问上游，且会为 Handler、MCP、测试及未来调用方增加网络等待。该 HIGH 风险已按 `WL-20260731-001` 明确豁免至 2026-08-30，届时必须将查询移至显式应用层步骤、调整到校验后，或拆分无副作用的 token setter。
- 当前 user ID 尚无生产消费者，身份查询失败会静默回退为仅保留 token；接入会话归属或用户级授权前，应明确失败策略与调用时机。
- 审阅阶段已执行 `go test ./...`、`go vet ./...` 与 `git diff --check`，均通过。

### 建议 Commit Message（git-cz）

- `feat(auth): resolve user ID from access token`

## CHANGELOG - 2026-07-30 20:54 - 在聊天上下文中注入当前授权学生资料

### 撰写时间

- 2026-07-30 20:54

### Base Commit

- 0063a3f679487c701a9ef2e1c292a938fcef5e6d

### Compare Scope

- working_tree_only

### 背景与改动目标

- Agent 原先只获得用户问题、动态日期提醒和 Skill catalog，无法在回答培养方案、年级、学院或在校状态等个性化问题时使用当前授权学生的基础资料。
- 本次接入同济开放平台的当前学生资料接口，并把数据限定为与本轮聊天绑定的上下文；同时保留已有请求级 access token 传递、Tool allowlist 和 SSE 生命周期。

### 改动概览

- 新增 `tongjiapi.UserInfo` 适配器，以当前请求的 Bearer token 调用 `/v1/rt/user/all_student`，校验开放平台业务码、响应 JSON 和非空数据后返回结构化资料；旧的通用 GET 入口与旧资料接口一并移除。
- 聊天服务仅在请求 context 含有效 access token 时加载资料；无 token 时维持原有聊天路径。资料加载失败会以 `user_info_unavailable` 终止当前 Run，避免在身份上下文缺失时继续执行。
- Runtime 新增 `StreamWithUserInfo`，将格式化资料追加到本轮动态提醒。资料文本通过 XML 转义置于 `user-profile-data/user-info` 数据边界，并明确声明其不是指令、不能改变 Tool 授权或安全策略。
- 补充开放平台请求、资料格式化、token 缺失、上游失败和恶意标签文本转义测试；审阅白名单增加“完整个人资料进入模型上下文”的临时豁免，失效时间为 2026-08-30。

### 关键链路解析（含上下游）

- 上游依赖：HTTP Chat/ChatStream 继续从 Authorization Header 提取 Bearer token 并写入 request context。`chat.Service` 从该 context 读取 token，按需调用 Tongji Open Platform，不读取全局或跨请求凭据。
- 当前改动：`loadFormattedUserInfo` 将开放平台返回的 `UserInfo` 格式化为稳定字段文本；`Runtime.StreamWithUserInfo` 调用输入构造器，使用 XML 转义后的资料建立与用户 query 分离的提醒消息，再运行 DeepAgent。
- 下游影响：模型在已授权请求中可参考当前学生资料；静态 Skill、远程 MCP Tool、RunState、流式事件和默认无 token 聊天行为保持不变。开放平台资料服务或授权 token 不可用时，本轮聊天会明确失败而不会调用模型。

### 改动结果与业务影响

- Agent 可根据当前用户的年级、学院、培养层次与在校状态提供更贴合的校园问题回答，且资料读取与聊天请求的取消/超时 context 保持一致。
- XML 转义阻止资料字段中的标签文本闭合或伪造资料边界；测试覆盖 `</user-info><instruction>...` 被保留为文本实体的场景。
- 已执行 `go test ./...`、`go vet ./...` 与 `git diff --check`，均通过。测试使用 fake Agent 和自定义 HTTP Transport，不访问真实模型、校园平台或宿主机 Shell。

### 风险与待办

- 当前 `FormatUserInfo` 会向模型提供包含生日、地址、学号等完整资料，该数据处理范围已登记为临时豁免，必须在 2026-08-30 前复核并按实际回答需求最小化字段。
- 每个带 token 的聊天请求都会新增一次开放平台调用；资料服务不可用会阻止本轮聊天。后续可根据产品可用性要求评估短期缓存、降级回答或显式的用户资料开关。
- XML 转义保护消息结构，但模型安全仍需依赖 Tool allowlist、参数校验与授权边界；未来接入更多外部资料时，必须继续将其标为非指令数据并增加拒绝场景测试。

### 建议 Commit Message（git-cz）

- `feat(chat): add authorized student context`

## CHANGELOG - 2026-07-30 18:17 - 将 Skill 加载状态隔离到单次 Agent Run

### 撰写时间

- 2026-07-30 18:17

### Base Commit

- f6d05cc468d15c2d64b958069182b323a04d558e

### Compare Scope

- working_tree_only

### 背景与改动目标

- `system.load_skill` 此前每次调用都会重新返回完整手册。需要在不扩大 Skill allowlist 和文件访问边界的前提下，避免同一 Agent Run 重复加载相同内容，并确保状态不会泄漏到下一次请求。
- 原系统工具实现与注册逻辑位于同一包，新增按 Run 状态的协作后需要拆分 Tool 实现与注册入口，同时保持聊天服务的静态 Tool 注册契约可用。

### 改动概览

- 新增 `skills.RunState`，以互斥锁维护单次 Run 已成功加载的 Skill；首次加载返回手册，重复加载返回稳定的 `already_loaded` 状态，加载失败不记录并允许后续重试。
- `Runtime.Stream` 为每次执行创建新的 RunState 并写入派生 context，再将该 context 同时交给 Runner 与 `Run`，使静态 Tool 能获得与当前请求绑定的状态。
- `system.load_skill` 移入独立子包，保留原有 Tool/Skill allowlist、严格 JSON 参数校验和嵌入手册读取边界；未携带 RunState 的调用改为返回 `skill_run_unavailable`，避免脱离 Agent Run 使用该工具。
- 更新系统 Tool 注册和聊天服务测试的导入路径，并补充 RunState、重复加载、失败重试、跨 Run 隔离及缺失状态的离线测试。

### 关键链路解析（含上下游）

- 上游依赖：HTTP 聊天入口仍将请求 context 传递到 `chat.Service` 与 Runtime；Runtime 在构建动态输入后派生该 context，不修改取消、deadline 或请求级鉴权信息。
- 当前改动：DeepAgent 在一次 `Runtime.Stream` 内调用 `system.load_skill` 时，Tool 通过 context 取得 RunState，并经 allowlist 校验后调用嵌入式 `skills.Load`。同一 Skill 成功加载后不再重复返回完整内容。
- 下游影响：模型仍只可发现和调用应用 allowlist 中的静态 Tool；聊天服务测试已同步使用拆分后的 `load_skill` 包常量，避免包重构导致编译断链。不同 Run 使用独立状态，互不共享已加载标记。

### 改动结果与业务影响

- 单轮多次请求同一 Skill 不会重复增加模型上下文，降低无效 Token 消耗；首次失败仍可恢复重试，避免短暂读取错误永久阻断该 Run。
- RunState 仅存储允许的 Skill ID，不持久化手册内容或用户数据；状态在每次 `Runtime.Stream` 新建，适用于并发请求隔离。
- 已执行 `go test ./...`、`go vet ./...` 与 `git diff --check`，均通过。测试仅使用嵌入式资源和 fake Agent，不访问真实模型、校园服务或宿主机 Shell。

### 风险与待办

- 当前重复加载返回状态而非手册全文，依赖模型已保留首次 Tool 结果；若未来引入 Tool 结果裁剪、跨节点上下文压缩或 Run 恢复，需要明确该状态与实际模型上下文的一致性策略。
- `RunState.LoadOnce` 为保证同一 Skill 不重复读取而持锁执行 loader。现有嵌入式读取很短；若 loader 未来变为网络或重 I/O，应改为按 Skill 粒度的并发控制，避免无关 Skill 串行等待。
- `system.load_skill` 的直接调用现在必须携带 RunState。未来新增非 Runtime 的调用入口时，应显式创建受控状态或维持当前拒绝语义，不能绕过单 Run 隔离。

### 建议 Commit Message（git-cz）

- `feat(agent): isolate skill loading per run`

## CHANGELOG - 2026-07-30 17:26 - 以开关受控恢复 Agent 文件系统 middleware

### 撰写时间

- 2026-07-30 17:26

### Base Commit

- 914f07e1f9b20918e2b7f1c201baff8ed6b4cdbc

### Compare Scope

- working_tree_only

### 背景与改动目标

- DeepAgent Runtime 已具备标准 Handler 扩展点；本次需要让开发环境可按配置接入文件系统 middleware，同时使默认聊天链路继续不具备宿主机文件或 Shell 能力。
- 本地 Backend 不能作为公开部署的安全沙箱。远程 AgentKit 沙箱替换完成前，部署约定要求 `SANDBOX_ENABLED` 始终为 `false`，并将该风险登记为有时限的审查豁免。

### 改动概览

- `runtime.Config` 新增并透传 `Handlers` 到 `deep.New`，使应用层可以在不改变 Runtime 主循环、Tool 配置和事件投影的前提下注入 Eino middleware。
- `chat.NewFromEnv` 在模型、知识库和受 allowlist 约束的 MCP Tool 初始化后读取 `SANDBOX_ENABLED`：关闭时传入空 Handler 列表；开启时创建文件系统 middleware。配置非法或 middleware 创建失败时关闭已创建的 MCP Client 后返回错误。
- 本地 sandbox 适配代码标注后续替换为 Ark AgentKit 远程沙箱；审查白名单新增临时豁免，匹配本地 Backend 空配置路径，失效时间为 2026-08-30。
- 新增离线测试，验证文件系统 middleware 可完成 Backend 装配并在 `BeforeAgent` 阶段注入工具，测试不执行本机 Shell。

### 关键链路解析（含上下游）

- 上游依赖：进程环境提供 `SANDBOX_ENABLED`，`sandbox.EnabledFromEnv` 负责解析并在非法值时返回可定位错误；模型、知识库和 MCP 初始化顺序保持不变。
- 当前改动：聊天服务将条件创建的 `adk.ChatModelAgentMiddleware` 放入 `runtime.Config`，Runtime 原样传给 `deep.Config.Handlers`。启用时 filesystem middleware 在 Agent 执行前追加文件系统工具；关闭时不会注册此类工具。
- 下游影响：DeepAgent 的既有主 Agent、静态 `system.load_skill`、远程 MCP Tool、流式事件和 12 次迭代上限均保持不变。只有显式启用开关的实例才获得本地文件与 Shell 相关能力。

### 改动结果与业务影响

- 默认配置继续关闭 sandbox，公开环境的 Agent 能力面不因本次改动扩大；受控开发环境可以通过开关验证文件处理链路，无需改动 Runtime 实现。
- Sandbox 初始化异常不会遗留已建立的 MCP Client；新增测试覆盖 middleware 的装配和工具注入结果。
- 已执行 `go test ./internal/integration/sandbox ./internal/application/chat ./internal/agentic/runtime` 与 `go vet ./internal/integration/sandbox ./internal/application/chat ./internal/agentic/runtime`，均通过。

### 风险与待办

- `SANDBOX_ENABLED=true` 会使用本地 Backend，能够访问宿主机文件系统并通过 Shell 执行命令；在 AgentKit 远程沙箱替换完成前，部署配置必须保持该变量为 `false`。
- 白名单豁免仅覆盖当前过渡期，须在 2026-08-30 前复核；替换后应移除本地 Backend、对应豁免和过渡性 TODO。
- 新增测试只验证离线装配与工具注册，不调用本机 Shell；远程沙箱接入时需补充其隔离边界、权限策略和失败清理的集成测试。

### 建议 Commit Message（git-cz）

- `feat(agent): gate filesystem middleware by sandbox config`

## CHANGELOG - 2026-07-30 12:25 - 收敛单 Agent 运行时并固化 SSE 事件契约

### 撰写时间

- 2026-07-30 12:25

### Base Commit

- a4d3db7cf063dd56d3c9d727d2dabdf7fb1c3bde

### Compare Scope

- staged_changes_only

### 背景与改动目标

- 当前服务只暴露经 allowlist 管理的 `system.load_skill` 与远程校园 MCP Tool，没有具备独立职责、权限或上下文契约的专用子 Agent。本地文件系统与 Shell middleware 也不应出现在公开 Agent 调用链中。
- SSE 已可投影 Run 生命周期，但事件 `data` 仍是松散 map，且终态后没有统一阻止继续发送的机制，难以稳定地对接客户端状态机。

### 改动概览

- Runtime 收敛为 DeepAgent 的标准模型—工具循环：统一关闭内置待办和通用子 Agent，移除应用层可变的 middleware/Agent 编排参数，并将单轮最大模型迭代显式设为 12。
- 聊天服务不再读取 `SANDBOX_ENABLED` 或装配宿主机文件系统、Shell middleware；远程 MCP Tool、静态 `system.load_skill`、Cozeloop 全局回调和请求级 token 透传保持不变。
- `internal/agentic/event` 为每类对外 SSE 事件新增具名数据结构，并让 Emitter 在发送 `run.completed` 或 `run.failed` 后拒绝后续事件，保证单个 Run 只有一个终态。
- 服务未初始化时，`chat.Stream` 与 `Service.Stream` 也会发送 `run.started` 后以 `agent_unavailable` 的 `run.failed` 收尾，使流式调用拥有完整可解析的生命周期。
- README、开发说明和风险文档同步运行时边界、12 次迭代上限、SSE `data` 契约与终态规则；移除对已不进入聊天调用链的 Sandbox 开关描述。

### 关键链路解析（含上下游）

- 上游依赖：HTTP `ChatStream` 继续创建请求专属可取消 context，并将 Emitter 生成的事件序号写入 SSE `id`。Cozeloop 继续通过 Eino 全局 callback 提供观测，不依赖聊天 Runtime middleware。
- 当前改动：`chat.NewFromEnv` 将已批准的静态与 MCP Tool 传入 Runtime；Runtime 用 `deep.New` 建立单 Agent，并以 12 次模型—工具循环作为单轮资源上限。服务层在 Runtime 前后投影 `run.started`、阶段状态和唯一终态。
- 下游影响：客户端可使用 `run_id`、`seq`/SSE `id` 及具名 `data` 字段驱动界面，收到终态后无需等待额外事件。任何公开调用均不会获得宿主机文件、命令执行或通用 `task` 子代理能力。

### 改动结果与业务影响

- 运行时能力面缩小为主 Agent 直接调用受批准 Tool，避免无专长通用子 Agent 带来的额外模型成本、延迟和上下文转交。
- SSE 对外协议从约定式 map 收敛为可维护的字段契约，并对服务不可用与正常执行失败提供一致的失败终态。
- 已执行 `go test ./...`、`go test -race ./...`、`go vet ./...` 与 `git diff --cached --check`，均通过。测试不读取 `.env`，不调用真实模型、校园平台或远程 MCP。

### 风险与待办

- 远程 MCP 与同济开放平台仍承担 token 有效性、用户绑定和 scope 审核；当前 Agent 仅在请求内透传格式正确的 Bearer token，不能替代身份鉴权。
- 当前 SSE 不支持断线重连、心跳、跨请求取消或 HITL Resume。客户端断开会取消当前 Run，但不会持久化或恢复中间状态。
- `internal/integration/sandbox` 仍保留为未接入聊天链路的本地适配代码；后续若重新引入文件处理，应替换为受控文件服务或隔离 Sandbox Adapter，而非恢复宿主机 Shell。

### 建议 Commit Message（git-cz）

- `refactor(agent): harden runtime and SSE contract`

## CHANGELOG - 2026-07-30 02:05 - 将每轮 Agent 输入收敛为动态提醒与结构化请求

### 撰写时间

- 2026-07-30 02:05

### Base Commit

- 6fb3aecba9150cfd0ce74592517284ae6af7c1d0

### Compare Scope

- working_tree_only

### 背景与改动目标

- 先前 Skill catalog 被拼接进静态 System Prompt，导致运行时日期与按轮提示无法独立更新，也使用户原始请求与运行时附加提醒混在同一段文本中。
- 当前 Agent 只有主 Agent、静态系统 Tool 和远程 MCP Tool，没有定义具备独立职责或权限边界的专用子代理；保留 Eino 的通用子代理会增加额外模型调用，并使按轮 catalog 难以传递到转交任务中。

### 改动概览

- 新增 `internal/agentic/runtime/input.go`。每次 `Runtime.Stream` 根据当前 `Etc/GMT-8` 时间构建 `<system-reminder>`，并在其中按需附加经过启动校验的 Skill catalog；原始用户 query 独立以 XML `interaction_request/user_query` 消息传入，特殊字符由 XML 编码。
- Runtime 配置新增 `SkillCatalog`，聊天服务在启动时构建一次 catalog 并传入 Runtime；`loadSystemInstruction` 只负责 CozeLoop PromptHub 的 System 指令，不再拼接 catalog。
- 调用 Runner 时由单字符串 `Query` 改为显式消息列表 `Run`，从而保留“动态提醒 + 结构化用户请求”的消息边界。
- 关闭 Eino 内置通用子代理（`WithoutGeneralSubAgent: true`），移除未定义专用职责的 `task` 转交能力；主 Agent 仍直接拥有 `system.load_skill` 和远程 MCP Tool。
- 新增输入构造测试，并更新聊天服务测试，验证未启用 CozeLoop 时静态 System Prompt 为空、Skill catalog 由本轮提醒消息承载。

### 关键链路解析（含上下游）

- 上游依赖：HTTP Handler 继续将请求体中的 `message` 作为单轮 query 传给 `chat.Service`；Cozeloop 启用时仍在启动阶段取得 System Prompt，Skill allowlist、manifest 与嵌入手册也仍在启动阶段校验。
- 当前改动：`chat.NewFromEnv` 将 catalog 保存进 `runtime.Config`。每次 `Runtime.Stream` 调用 `buildInputMessagesWithHistory`，先发送包含当前日期和 catalog 的提醒消息，再发送 XML 转义后的用户请求，随后使用 `Runner.Run` 执行 Deep Agent。
- 下游影响：模型可在每轮获得当前日期与安全的 Skill 发现信息，同时用户 query 不与运行时提醒直接拼接。因为不再注册通用子代理，复杂请求仍由主 Agent 在同一 Run 中顺序调用已批准 Tool，不会进入缺少 catalog 的泛用子 Agent。

### 改动结果与业务影响

- 动态信息从服务启动期移动到请求执行期；同一长期运行实例中的日期提醒不会因启动时间陈旧而失效。
- 文档生成、文档优化和成绩查询仍由同一个受 allowlist 约束的主 Agent 执行，避免无专长子代理带来的额外延迟、成本和上下文转交损耗。
- 新增测试覆盖时区日期、catalog 注入、XML 特殊字符编码以及静态 Prompt 与 catalog 的职责分离。

### 风险与待办

- 当前输入消息中的 `<system-reminder>` 使用 User role 承载，其内容是运行时生成的提示而非访问控制边界；Tool 和 Skill 的实际授权仍必须依赖既有 allowlist 与参数校验，不能依赖模型遵守提醒文本。
- 关闭通用子代理后不再具备内置 `task` 的隔离、并行与长任务委派能力。将来若引入专用子代理，应为其定义 Tool 范围、上下文传递、取消与结果汇总契约，并显式传递需要的 Skill catalog。
- `doc-generator` 手册仍要求基于源码与参考文档完成事实核验；公开部署默认关闭 Sandbox，因此该类请求需要在未来明确提供受控的只读工程上下文，不能假定 Agent 一定能访问本地文件。

### 建议 Commit Message（git-cz）

- `refactor(agent): structure per-turn runtime input`

## CHANGELOG - 2026-07-30 01:38 - 为 Agent 引入受控的按需 Skill 加载

### 撰写时间

- 2026-07-30 01:38

### Base Commit

- da7a682eef905d1a2b530fd76edc2b350bb62b6c

### Compare Scope

- staged_changes_only

### 背景与改动目标

- Agent 需要能够在用户请求文档生成或优化时获得具体工作手册，但把完整手册直接写入初始 System Prompt 会增加上下文开销，并使所有能力的说明无条件暴露给模型。
- 本次新增仅由应用 allowlist 管理的 Skill 目录与宿主静态 Tool：初始 Prompt 只注入安全摘要；模型在满足触发条件时，才能通过 `system.load_skill` 读取已批准的完整手册。

### 改动概览

- 新增嵌入式 `internal/agentic/skills` 仓库，将 `doc-generator` 与新增的 `doc-optimizer` 手册随二进制发布。读取接口只接受安全的 Skill ID、固定读取 `SKILL.md`，并限制单份内容为 64 KiB。
- 新增 Skill manifest 和 catalog。Catalog 根据 Skill allowlist 排序生成不含本地路径与手册正文的摘要，并在服务启动时校验每个已批准 Skill 都有合法 manifest 和可读取手册。
- 新增静态 `system.load_skill` Tool，严格校验参数与 Tool/Skill allowlist；仅返回固定主手册或脱敏的稳定状态，不允许调用方读取任意文件。
- Tool allowlist 分为静态系统 Tool 与远程 MCP Tool；聊天服务将已批准的静态 Tool 与远程 MCP Tool 一同注册，并把 Skill catalog 追加到从环境或 PromptHub 取得的 System Prompt。
- 补充 catalog、嵌入资源、静态 Tool 注册和聊天 Prompt 测试；测试覆盖新增 `doc-optimizer` 以及已批准 Skill 缺少 manifest 时阻止启动的路径。

### 关键链路解析（含上下游）

- 上游依赖：`chat.NewFromEnv` 仍负责初始化模型、知识库和远程 MCP；随后读取远程 `toolallowlist.MCPTools()`，并取得 `systemtools.Tools()` 中的本地静态 Tool。
- 当前改动：`loadSystemInstruction` 在取得空指令或 PromptHub 指令后调用 `skills.Catalog()`。Catalog 只写入 Skill ID、触发条件与 `system.load_skill` 使用约束；Agent 实际调用 Tool 后，Tool 再从嵌入文件系统读取对应 `SKILL.md`。
- 下游影响：Deep Agent 可在单次 Run 内按需使用文档生成与文档优化手册，但不能通过 Tool 探测未批准 Skill、路径或任意嵌入资源；远程 MCP 的成绩 Tool 注册和请求级 token 透传链路保持不变。

### 改动结果与业务影响

- 文档类能力从“全部手册始终进 Prompt”收敛为“安全目录常驻、完整手册按需加载”，降低默认上下文占用并保留明确的能力发现入口。
- `doc-optimizer` 成为显式批准的 Skill；其 manifest 指明应在文档生成完成后或用户要求不改变原意的润色、优化、重写时加载。
- 已执行 `go test ./...`、`go test -race ./...`、`go vet ./...`、`git diff --check` 与 `git diff --cached --check`，均通过。测试仅使用嵌入资源与本地 fake Tool，不读取 `.env`，不调用真实模型、校园平台或远程 MCP。

### 风险与待办

- Skill 内容已被编译进二进制；新增或修改手册需要重新构建、发布，并同步维护 allowlist、manifest 与测试，否则启动时会因完整性校验失败。
- 当前 Runtime 仍设置 `WithoutWriteTodos: true`，但本次仅新增 `system.load_skill`，未注册其注释中提到的任务计划替代 Tool。若依赖多步骤执行计划，需要后续单独补齐或恢复内置待办能力。
- `system.load_skill` 返回完整手册给模型；未来新增包含敏感示例、凭据或外部链接的手册前，应先审查其内容边界和长度限制。

### 建议 Commit Message（git-cz）

- `feat(agent): add allowlisted on-demand skill loading`

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
