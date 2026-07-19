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
