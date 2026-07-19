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
