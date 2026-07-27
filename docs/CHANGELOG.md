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
