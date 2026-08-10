# TongjiStudent

TongjiStudent 是一个面向同济大学校园场景的 Agent 服务基架。项目使用开源 [CloudWeGo Hertz](https://www.cloudwego.io/zh/docs/hertz/) 提供 HTTP 服务，使用 [CloudWeGo Eino](https://github.com/cloudwego/eino) 构建 DeepAgent，并内置一个可替换的 MCP Server 示例。

目前项目已完成基础运行时、模型配置、DeepAgent、MCP 工具和多轮会话 HTTP 接口。

## 当前能力

- 开源 Hertz HTTP 服务，默认监听 `8080` 端口。
- 健康检查接口：`GET /v1/ping`。
- Agent 调用接口：创建会话后，通过 `POST /v1/sessions/:session_id/messages` 以 SSE 执行并持久化每轮对话。
- 基于 Ark 兼容配置初始化 Eino Agent Graph。
- 启动时连接远程 Streamable HTTP MCP Server，并只向 Agent 暴露 allowlist 中的工具。
- 可选 Cozeloop 集成，用于 Trace 观测与系统 Prompt 管理；可视为开源版 Fornax。
- 本地日志模块，不直接使用项目业务代码中的字节内部日志库。

## 项目结构

```text
.
├── biz/handler/           # HTTP 处理函数与健康检查接口
├── internal/
│   ├── application/chat/  # 会话聊天应用服务与依赖装配
│   ├── agentic/runtime/   # 与具体模型、工具解耦的 DeepAgent 运行时封装
│   ├── integration/       # Ark、知识库、Cozeloop（开源版 Fornax）、MCP、本地 Sandbox 与同济开放平台适配
│   └── platform/          # 服务配置与日志等基础能力
├── script/                # 构建产物启动脚本
├── .env                   # 本地配置（已被 Git 忽略）
├── main.go                # Hertz 服务入口
├── router.go              # 自定义路由
└── router_gen.go          # Hertz 路由注册
```

## 前置条件

- Go `1.23.8`（项目的 `go.mod` 指定的 toolchain 版本）。
- 可访问项目 Go 依赖。
- 可用的模型 Endpoint 凭据与可访问的远程 MCP Server。
- 可访问的 PostgreSQL 与 Redis。服务会在启动时连接它们，并自动创建最小会话表结构；任一依赖不可用都会导致启动失败。

## 配置本地环境

在项目根目录创建或编辑 `.env`。该文件不会提交到 Git。

```env
# Ark/模型服务配置：ARK_BASE_URL 优先于 ARK_BASE_URL_CN
ARK_BASE_URL_CN=https://your-model-endpoint
ENDPOINT_ID=your-endpoint-id
ENDPOINT_API_KEY=your-api-key

# 可选 Cozeloop 集成（开源版 Fornax）：用于 Trace 观测与系统 Prompt 管理
COZELOOP_ENABLED=false
# COZELOOP_WORKSPACE_ID=your-workspace-id
# COZELOOP_JWT_OAUTH_CLIENT_ID=your-jwt-oauth-client-id
# COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID=your-public-key-id
# COZELOOP_JWT_OAUTH_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"

# Ark 知识库检索：启用后，Agent 可按需调用 system.search_knowledge 获取参考资料
ARK_KNOWLEDGE_ENABLED=false
# ARK_AK=your-knowledge-ak
# ARK_SK=your-knowledge-sk
# ARK_KNOWLEDGE_COLLECTION=your-collection-name
# ARK_KNOWLEDGE_PROJECT=default
# ARK_KNOWLEDGE_RESOURCE_ID=your-resource-id # 可替代 COLLECTION
# ARK_KNOWLEDGE_LIMIT=5
# ARK_KNOWLEDGE_DOMAIN=api-knowledgebase.mlp.cn-beijing.volces.com

# 远程 TongjiStudent MCP Server；启动时会连接、初始化并校验 allowlist
MCP_SERVER_URL=http://127.0.0.1:3000/mcp
MCP_TIMEOUT=12s

# 会话存储：认证会话使用 PostgreSQL，匿名会话使用 Redis；两项均为启动必填
POSTGRES_DSN=postgres://postgres:postgres@127.0.0.1:5432/tongji_student?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379/0
# SESSION_ANONYMOUS_TTL=24h
# SESSION_ANONYMOUS_MAX_MESSAGES=20
# SESSION_HISTORY_MAX_MESSAGES=20

# 同济开放平台 OAuth 2.0 授权码模式
TONGJI_OPEN_PLATFORM_CLIENT_ID=your-client-id
TONGJI_OPEN_PLATFORM_CLIENT_SECRET=your-client-secret
TONGJI_OPEN_PLATFORM_REDIRECT_URI=https://app.tongji.edu.cn/wallbreakerAuth/callback.html
TONGJI_OPEN_PLATFORM_STATE_SECRET=replace-with-a-random-secret
```

`ENDPOINT_ID`、`ENDPOINT_API_KEY`、`ARK_BASE_URL`（或 `ARK_BASE_URL_CN`）、`POSTGRES_DSN` 和 `REDIS_URL` 均为必填项。服务启动时会校验并连接模型、会话存储和远程 MCP。

如需启用 Cozeloop，请显式设置 `COZELOOP_ENABLED=true` 以及对应的 `COZELOOP_*` 变量。当前项目会用它注册 Eino 全局回调，并从 PromptHub 拉取 `prompt.tongjistudent.system_prompt` 作为系统提示词；它承担的是原先 Fornax 对应的观测与 Prompt 管理职责，但这里采用的是开源 Cozeloop 实现。

启用知识库时，必须配置 `ARK_AK`、`ARK_SK`，以及
`ARK_KNOWLEDGE_COLLECTION` 或 `ARK_KNOWLEDGE_RESOURCE_ID`。服务会注册只读的 `system.search_knowledge` 系统工具，Agent 仅在校园公开信息需要官方依据、时效性或适用范围核验时按需调用。工具结果以非可信参考资料返回；默认不展示来源，只有用户明确要求来源、依据或通知原文时，才可提供返回的来源标题。个人实时数据仍必须使用对应 Tongji MCP 工具。

## 同济开放平台浏览器授权

服务提供授权码模式的两个接口，客户端密钥和 state 签名密钥只从 `.env` 读取；`.env` 已被 Git 忽略，可直接参考本 README 中的配置示例。

| 方法   | 路径                         | 用途                                                                                       |
| ------ | ---------------------------- | ------------------------------------------------------------------------------------------ |
| `GET`  | `/v1/tongji/oauth/authorize` | 创建签名 `state`，并 302 跳转到同济统一认证页面。                                          |
| `POST` | `/v1/tongji/oauth/token`     | callback 页面提交 `code` 和 `state`，服务校验 state 后换取并返回短期 Bearer access token。 |

浏览器访问 `https://<你的 Go 服务域名>/v1/tongji/oauth/authorize` 后，开放平台会重定向到已登记的 `https://app.tongji.edu.cn/wallbreakerAuth/callback.html?code=...&state=...`。由于该回调页不在 Go 服务域名下，它必须将参数提交回 Go 服务：

```js
const params = new URLSearchParams(window.location.search);
const response = await fetch(
    "https://<你的 Go 服务域名>/v1/tongji/oauth/token",
    {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
            code: params.get("code"),
            state: params.get("state"),
        }),
    },
);
const bearerToken = await response.json();
```

该接口仅允许 `https://app.tongji.edu.cn` 跨域读取响应，响应包含 `access_token`、`token_type`、`expires_in` 和 `scope`，**不会**把 refresh token 返回给浏览器或写入日志。前端应把 access token 保存在内存中并在过期后重新授权；不得放入 URL、日志或长期 `localStorage`。

返回的 token 仅用于当前浏览器会话，服务不持久化它。会话接口可由浏览器以 HTTP `Authorization: Bearer <access_token>` 传入该短期 token；服务会在本次请求内尽力调用同济开放平台解析 `user_id`。解析成功时，该请求创建和访问 PostgreSQL 持久会话；缺失、格式错误或解析失败时，仍允许调用，但会退回 Redis 匿名会话。token 不会写入模型消息、响应或普通日志。

当前阶段仍未完成 token scope 审核，也没有面向匿名会话的设备绑定。前端必须在创建、提交消息、读取历史这三个请求中传入同一 Bearer token；不得把 token 或匿名 `session_id` 放入 URL、日志或长期 `localStorage`。

### 单测中的 API 调用 demo

默认测试使用本地 Fake Server，不访问真实开放平台：

```bash
go test ./internal/integration/tongjiapi
```

## 本地启动

推荐直接运行源码，`godotenv` 会自动加载项目根目录的 `.env`：

```bash
go mod download
go run .
```

默认端口为 `8080`。可通过环境变量或启动参数覆盖：

```bash
PORT0=8081 go run .
# 或
go run . -port=8081
```

需要构建二进制时：

```bash
go build -o bin/tongjistudent .
./bin/tongjistudent -port=8080
```

> 本地也可使用 `./local_run.sh [port]` 完成环境校验、依赖下载和启动；部署模板中的 `script/bootstrap.sh` 已移除。

## 验证服务

服务启动后，另开一个终端执行：

```bash
curl http://127.0.0.1:8080/v1/ping
```

三个接口都会返回 HTTP `200` 和 JSON 响应，例如：

```json
{ "message": "hey yo!" }
```

运行测试以验证远程 MCP 适配及其他已覆盖模块：

```bash
go test ./...
```

## 如何使用当前服务

当前 Agent 是**会话优先**的接口：先创建一个 `session_id`，每一轮再提交给这个会话。不要再调用旧的 `/v1/agent/chat` 或 `/v1/agent/chat/stream`，它们已不在路由中。

开始前，请确认以下服务和配置均已就绪：模型 Endpoint、远程 MCP Server、PostgreSQL、Redis，以及本 README 的 `.env` 必填项。然后启动本服务：

```bash
go run .
```

下面的示例假定服务运行在 `http://127.0.0.1:8080`，面向浏览器或前端服务接入，不再使用 `curl`。

### API 详细定义

#### `GET /v1/ping`

描述：健康检查接口，用于服务存活探测。

请求参数：

| 参数位置 | 参数名 | 类型 | 必填 | 描述       | 默认值 |
| -------- | ------ | ---- | ---- | ---------- | ------ |
| -        | -      | -    | -    | 无请求参数 | -      |

响应参数：

| 参数名    | 类型     | 必返 | 描述         | 默认值      |
| --------- | -------- | ---- | ------------ | ----------- |
| `message` | `string` | 是   | 固定响应文案 | `"hey yo!"` |

#### `GET /v1/tongji/oauth/authorize`

描述：创建签名 `state` 并重定向到同济统一认证页面，作为浏览器授权流程入口。

请求参数：

| 参数位置 | 参数名 | 类型 | 必填 | 描述       | 默认值 |
| -------- | ------ | ---- | ---- | ---------- | ------ |
| -        | -      | -    | -    | 无请求参数 | -      |

响应参数：

| 参数名     | 类型     | 必返 | 描述                                   | 默认值   |
| ---------- | -------- | ---- | -------------------------------------- | -------- |
| `Location` | `string` | 是   | 302 跳转目标 URL，指向同济统一认证页面 | 动态生成 |

#### `POST /v1/tongji/oauth/token`

描述：callback 页面提交 `code` 和 `state`，服务校验 state 后换取短期 Bearer access token。

请求参数：

| 参数位置 | 参数名         | 类型     | 必填 | 描述                                       | 默认值             |
| -------- | -------------- | -------- | ---- | ------------------------------------------ | ------------------ |
| Header   | `Content-Type` | `string` | 是   | 请求体编码类型                             | `application/json` |
| Body     | `code`         | `string` | 是   | 同济开放平台回调附带的授权码               | 无                 |
| Body     | `state`        | `string` | 是   | 授权阶段签发并回传的 state，用于防篡改校验 | 无                 |

响应参数：

| 参数名         | 类型     | 必返 | 描述                                  | 默认值         |
| -------------- | -------- | ---- | ------------------------------------- | -------------- |
| `access_token` | `string` | 是   | 当前浏览器会话使用的短期 Bearer token | 无             |
| `token_type`   | `string` | 是   | Token 类型                            | `"Bearer"`     |
| `expires_in`   | `number` | 是   | token 过期时间，单位秒                | 由开放平台决定 |
| `scope`        | `string` | 是   | 当前 token 的 scope                   | 由开放平台决定 |

#### `POST /v1/sessions`

描述：创建一个新会话；可创建匿名临时会话，也可在携带有效校园 Bearer token 时创建认证持久会话。

请求参数：

| 参数位置 | 参数名          | 类型     | 必填 | 描述                                               | 默认值 |
| -------- | --------------- | -------- | ---- | -------------------------------------------------- | ------ |
| Header   | `Authorization` | `string` | 否   | 认证会话传 `Bearer <access_token>`；匿名会话可不传 | 无     |

响应参数：

| 参数名        | 类型     | 必返 | 描述                                            | 默认值           |
| ------------- | -------- | ---- | ----------------------------------------------- | ---------------- |
| `session_id`  | `string` | 是   | 会话 ID；匿名会话通常为 `anon_` 前缀            | 动态生成         |
| `persistence` | `string` | 是   | 会话持久化类型，取值为 `ephemeral` 或 `durable` | 根据鉴权结果决定 |

#### `POST /v1/sessions/:session_id/messages`

描述：向指定会话提交一轮用户消息，并通过 SSE 持续返回本轮执行事件与回答增量。

请求参数：

| 参数位置 | 参数名          | 类型     | 必填 | 描述                                                                       | 默认值             |
| -------- | --------------- | -------- | ---- | -------------------------------------------------------------------------- | ------------------ |
| Path     | `session_id`    | `string` | 是   | 目标会话 ID                                                                | 无                 |
| Header   | `Content-Type`  | `string` | 是   | 请求体编码类型                                                             | `application/json` |
| Header   | `Authorization` | `string` | 否   | 认证会话必须继续传创建会话时的同一 `Bearer <access_token>`；匿名会话可不传 | 无                 |
| Body     | `message`       | `string` | 是   | 本轮用户输入，不能为空字符串                                               | 无                 |

响应参数：

响应为 `text/event-stream`，每条 SSE 事件的公共字段如下：

| 参数名             | 类型     | 必返 | 描述                                                           | 默认值                  |
| ------------------ | -------- | ---- | -------------------------------------------------------------- | ----------------------- |
| `id`               | `string` | 是   | 事件唯一 ID，当前与 `seq` 相同                                 | 按事件递增生成          |
| `event`            | `string` | 是   | 事件名称，如 `run.started`、`assistant.delta`、`run.completed` | 无                      |
| `data.run_id`      | `string` | 是   | 本次运行 ID                                                    | 动态生成                |
| `data.session_id`  | `string` | 是   | 所属会话 ID                                                    | 当前请求的 `session_id` |
| `data.seq`         | `number` | 是   | 当前运行内的事件序号，从 `1` 开始递增                          | 从 `1` 开始             |
| `data.occurred_at` | `string` | 是   | 事件发生时间，UTC 时间戳                                       | 动态生成                |

不同事件的 `data` 业务字段如下：

| 事件名                | 参数名         | 类型            | 必返 | 描述                                                                                                   | 默认值   |
| --------------------- | -------------- | --------------- | ---- | ------------------------------------------------------------------------------------------------------ | -------- |
| `run.started`         | `message`      | `string`        | 是   | Run 已接受并开始处理的提示文案                                                                         | 无       |
| `agent.status`        | `phase`        | `string`        | 是   | 当前执行阶段                                                                                           | 无       |
| `agent.status`        | `message`      | `string`        | 是   | 面向客户端展示的阶段描述                                                                               | 无       |
| `assistant.delta`     | `text`         | `string`        | 是   | 最终自然语言回答的增量文本                                                                             | 无       |
| `tool.call.started`   | `call_id`      | `string`        | 是   | 工具调用 ID                                                                                            | 动态生成 |
| `tool.call.started`   | `tool`         | `string`        | 是   | 工具标识                                                                                               | 无       |
| `tool.call.started`   | `display_name` | `string`        | 是   | 面向展示的工具名称                                                                                     | 无       |
| `tool.call.completed` | `call_id`      | `string`        | 是   | 工具调用 ID                                                                                            | 无       |
| `tool.call.completed` | `tool`         | `string`        | 是   | 工具标识                                                                                               | 无       |
| `tool.call.completed` | `duration_ms`  | `number`        | 是   | 本次工具调用耗时，单位毫秒                                                                             | 动态生成 |
| `tool.call.failed`    | `call_id`      | `string`        | 是   | 工具调用 ID                                                                                            | 无       |
| `tool.call.failed`    | `tool`         | `string`        | 是   | 工具标识                                                                                               | 无       |
| `tool.call.failed`    | `duration_ms`  | `number`        | 是   | 本次工具调用耗时，单位毫秒                                                                             | 动态生成 |
| `tool.call.failed`    | `code`         | `string`        | 是   | 工具失败码                                                                                             | 无       |
| `tool.call.failed`    | `message`      | `string`        | 是   | 工具失败说明                                                                                           | 无       |
| `task_plan.updated`   | `action`       | `string`        | 是   | 触发更新的任务计划操作                                                                                 | 无       |
| `task_plan.updated`   | `revision`     | `number`        | 是   | 最新任务计划版本                                                                                       | 动态生成 |
| `task_plan.updated`   | `tasks`        | `array<object>` | 是   | 当前任务计划完整快照                                                                                   | `[]`     |
| `run.completed`       | `duration_ms`  | `number`        | 是   | 本次运行总耗时，单位毫秒                                                                               | 动态生成 |
| `run.failed`          | `code`         | `string`        | 是   | 失败码，如 `turn_in_progress`、`session_unavailable`、`session_write_failed`、`agent_execution_failed` | 无       |
| `run.failed`          | `message`      | `string`        | 是   | 失败说明                                                                                               | 无       |

#### `GET /v1/sessions/:session_id/messages`

描述：读取指定会话最近历史消息，结果按时间和消息序号从旧到新排列。

请求参数：

| 参数位置 | 参数名          | 类型     | 必填 | 描述                                                                   | 默认值 |
| -------- | --------------- | -------- | ---- | ---------------------------------------------------------------------- | ------ |
| Path     | `session_id`    | `string` | 是   | 目标会话 ID                                                            | 无     |
| Query    | `limit`         | `number` | 否   | 返回消息条数，取值范围 `1..100`                                        | `20`   |
| Header   | `Authorization` | `string` | 否   | 认证会话继续传创建会话时的同一 `Bearer <access_token>`；匿名会话可不传 | 无     |

响应参数：

| 参数名                 | 类型            | 必返 | 描述                             | 默认值         |
| ---------------------- | --------------- | ---- | -------------------------------- | -------------- |
| `messages`             | `array<object>` | 是   | 消息列表                         | `[]`           |
| `messages[].ID`        | `string`        | 是   | 消息 ID                          | 动态生成       |
| `messages[].SessionID` | `string`        | 是   | 所属会话 ID                      | 当前会话 ID    |
| `messages[].Sequence`  | `number`        | 是   | 会话内消息序号                   | 按存储顺序生成 |
| `messages[].Role`      | `string`        | 是   | 消息角色，如 `user`、`assistant` | 无             |
| `messages[].Content`   | `string`        | 是   | 消息文本内容                     | 无             |
| `messages[].CreatedAt` | `string`        | 是   | 消息创建时间，UTC 时间戳         | 动态生成       |

#### `GET /v1/sessions/:session_id/task-plan`

描述：读取指定会话当前活动任务计划；认证会话按当前 `user_id` 校验归属，匿名会话按 `session_id` capability 校验。当前不存在活动计划时返回 `{"plan":null}`。

| 参数位置 | 参数名          | 类型     | 必填 | 描述                                                                       | 默认值 |
| -------- | --------------- | -------- | ---- | -------------------------------------------------------------------------- | ------ |
| Path     | `session_id`    | `string` | 是   | 目标会话 ID                                                                | 无     |
| Header   | `Authorization` | `string` | 否   | 认证会话必须继续传创建会话时的同一 `Bearer <access_token>`；匿名会话可不传 | 无     |

响应体中的 `plan` 为 `null` 或包含 `session_id`、`revision`、`tasks`、`updated_at` 的任务计划快照。

### 推荐调用顺序

| 场景         | 调用顺序                                                                                                                                                                                      |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 匿名会话     | `POST /v1/sessions` -> `POST /v1/sessions/:session_id/messages` -> `GET /v1/sessions/:session_id/messages` / `task-plan`                                                                      |
| 认证持久会话 | `GET /v1/tongji/oauth/authorize` -> `POST /v1/tongji/oauth/token` -> `POST /v1/sessions` -> `POST /v1/sessions/:session_id/messages` -> `GET /v1/sessions/:session_id/messages` / `task-plan` |

### 1. 发起同济开放平台授权

浏览器应直接跳转到授权地址，而不是用 AJAX 调用：

```js
const API_BASE = "http://127.0.0.1:8080";

window.location.href = `${API_BASE}/v1/tongji/oauth/authorize`;
```

开放平台完成登录后，会跳回已登记的 callback 页面 `https://app.tongji.edu.cn/wallbreakerAuth/callback.html?code=...&state=...`。

### 2. callback 页面换取 access token

callback 页面需要读取 URL 中的 `code` 和 `state`，再提交给 Go 服务：

```js
const API_BASE = "http://127.0.0.1:8080";
const params = new URLSearchParams(window.location.search);

const tokenResponse = await fetch(`${API_BASE}/v1/tongji/oauth/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
        code: params.get("code"),
        state: params.get("state"),
    }),
});

if (!tokenResponse.ok) {
    throw new Error(`token exchange failed: ${tokenResponse.status}`);
}

const tokenPayload = await tokenResponse.json();
const accessToken = tokenPayload.access_token;
```

返回示例：

```json
{
    "access_token": "<token>",
    "token_type": "Bearer",
    "expires_in": 7200,
    "scope": "..."
}
```

该 token 只建议保存在当前浏览器会话内存中；不要放入 URL、日志或长期 `localStorage`。

### 3. 创建会话

#### 匿名会话

不传 `Authorization` 即可创建匿名会话：

```js
const API_BASE = "http://127.0.0.1:8080";

const sessionResponse = await fetch(`${API_BASE}/v1/sessions`, {
    method: "POST",
});

if (!sessionResponse.ok) {
    throw new Error(`create session failed: ${sessionResponse.status}`);
}

const sessionPayload = await sessionResponse.json();
const sessionId = sessionPayload.session_id;
```

成功返回：

```json
{ "session_id": "anon_<random>", "persistence": "ephemeral" }
```

匿名会话保存在 Redis。默认 24 小时无活动后过期，每个会话最多保留最近 20 条消息；可用 `SESSION_ANONYMOUS_TTL` 与 `SESSION_ANONYMOUS_MAX_MESSAGES` 调整。`session_id` 是匿名会话的访问能力，获得它的调用方能够继续提交和读取该会话，因此应像短期凭据一样保管。

#### 认证持久会话

认证会话需要在创建、提交消息、读取历史这三个请求中持续携带同一 Bearer token：

```js
const API_BASE = "http://127.0.0.1:8080";
const accessToken = "<access_token>";

const sessionResponse = await fetch(`${API_BASE}/v1/sessions`, {
    method: "POST",
    headers: {
        Authorization: `Bearer ${accessToken}`,
    },
});

if (!sessionResponse.ok) {
    throw new Error(`create session failed: ${sessionResponse.status}`);
}

const sessionPayload = await sessionResponse.json();
const sessionId = sessionPayload.session_id;
const persistence = sessionPayload.persistence;
```

服务能够从该 token 解析到 `user_id` 时，返回的 `persistence` 为 `durable`，会话及消息保存到 PostgreSQL，并按 `user_id` 限制读取与写入。如果 token 无效、无法解析用户 ID 或未传 token，服务不会拒绝创建请求，而是返回 `ephemeral` 匿名会话；客户端应检查 `persistence`，不要把匿名会话误当成可恢复的用户会话。

### 4. 提交一轮消息并消费 SSE

`POST /v1/sessions/:session_id/messages` 是唯一的 Agent 执行入口，请求体只接受非空的 `message`。下面示例使用浏览器 `fetch` 读取 `text/event-stream`：

```js
const API_BASE = "http://127.0.0.1:8080";
const sessionId = "<session_id>";
const accessToken = "<access_token>"; // 匿名会话可省略

const response = await fetch(`${API_BASE}/v1/sessions/${sessionId}/messages`, {
    method: "POST",
    headers: {
        "Content-Type": "application/json",
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    },
    body: JSON.stringify({ message: "现在几点了？" }),
});

if (!response.ok || !response.body) {
    throw new Error(`send message failed: ${response.status}`);
}

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    const chunk = decoder.decode(value, { stream: true });
    console.log(chunk);
}
```

匿名会话可省略 `Authorization`。认证会话则必须继续携带创建时的同一 token，否则服务会按匿名会话查询，通常得到终态 `run.failed`，错误码为 `session_unavailable`。

一轮处理的顺序是：读取最近历史 -> 保存当前 user 消息 -> 执行 Agent/Tool -> 成功时保存最终 assistant 文本。若模型执行失败，user 消息仍会保留，但不会写入虚假的 assistant 回复；客户端读取历史时应允许最后一条消息只有 user 角色。

### 5. 读取最近历史

```js
const API_BASE = "http://127.0.0.1:8080";
const sessionId = "<session_id>";
const accessToken = "<access_token>"; // 匿名会话可省略

const historyResponse = await fetch(
    `${API_BASE}/v1/sessions/${sessionId}/messages?limit=20`,
    {
        headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
    },
);

if (!historyResponse.ok) {
    throw new Error(`load history failed: ${historyResponse.status}`);
}

const historyPayload = await historyResponse.json();
console.log(historyPayload.messages);
```

`limit` 默认是 `20`，可取 `1` 到 `100`。结果按时间和消息序号从旧到新排列；当前返回结构直接使用 Go 领域对象的字段名：

```json
{
    "messages": [
        {
            "ID": "msg_<random>",
            "SessionID": "ses_<random>",
            "Sequence": 1,
            "Role": "user",
            "Content": "现在几点了？",
            "CreatedAt": "2026-08-05T00:00:00Z"
        }
    ]
}
```

会话不存在、已过期，或认证会话不属于当前 `user_id` 时，接口返回 HTTP `404` 和 `{"error":"session not found"}`。非法 `limit`、空 `session_id` 或空/非法消息返回 HTTP `400`。

### SSE 事件与失败处理

提交消息后，HTTP 响应为 `200` 的 `text/event-stream`。业务执行失败不会再切换 HTTP 状态码，而是通过终态 `run.failed` 事件返回，例如并发提交同一会话时的 `turn_in_progress`、会话不可用时的 `session_unavailable`、会话写入失败时的 `session_write_failed`，或模型运行失败时的 `agent_execution_failed`。创建会话的基础设施不可用则直接返回 HTTP `503`。

响应会包含用于问题排查的 `X-Request-ID`；普通日志只记录该 ID、方法、路径、状态码和耗时，不记录请求或响应内容。

所有 SSE 事件都包含同一次运行的 `run_id`、所属 `session_id`、从 `1` 开始递增的 `seq` 和 UTC `occurred_at`；`id` 与 `seq` 相同，可供客户端去重。当前协议会发送模型 reasoning、工具参数和工具结果，前端必须按会话归属处理这些内容；事件不会包含 Bearer token、数据库连接串或其他服务端凭据。

| 事件                  | `data` 契约                                         | 含义                                                   |
| --------------------- | --------------------------------------------------- | ------------------------------------------------------ |
| `run.started`         | `message`                                           | Run 已接受并开始处理。                                 |
| `agent.status`        | `phase`, `message`                                  | 可展示的执行阶段。                                     |
| `assistant.delta`     | `text`                                              | 最终自然语言回答的增量文本。                           |
| `tool.call.started`   | `call_id`, `tool`, `display_name`                   | 模型已选择该工具，调用即将执行。                       |
| `tool.call.completed` | `call_id`, `tool`, `duration_ms`                    | Agent 已收到工具结果；不代表上游业务一定成功。         |
| `tool.call.failed`    | `call_id`, `tool`, `duration_ms`, `code`, `message` | 调用执行失败；Agent 会终止本轮或接收稳定错误结果。     |
| `task_plan.updated`   | `action`, `revision`, `tasks`                       | 当前会话任务计划已更新；前端应以完整快照刷新进度面板。 |
| `run.completed`       | `duration_ms`                                       | Run 成功结束。                                         |
| `run.failed`          | `code`, `message`                                   | Run 无法完成。                                         |

`run.completed` 与 `run.failed` 是互斥的终态事件：每个 Run 必须且只能发送其中一个，终态事件后不再发送其他事件。当前接口不支持断线重连、心跳、跨请求取消或 HITL Resume；客户端断开时服务会取消仍在执行的本次 Run。

可使用 `GET /v1/sessions/:session_id/messages?limit=20` 读取当前请求有权访问的 canonical 历史消息，或使用 `GET /v1/sessions/:session_id/task-plan` 在页面刷新后恢复当前任务计划。

## 模型与 MCP 的现状

启动过程中会依次创建 Ark Responses API 模型客户端、远程 Streamable HTTP MCP Client、allowlist 中的 MCP 工具和 DeepAgent。因此，服务能成功启动代表模型配置格式、远程 MCP 初始化及允许工具发现均已通过。

主 Agent 固定启用 Ark Responses API 的 response-chain 会话缓存，TTL 为 600 秒。每轮 Agent 输出的 `response_id` 和缓存到期时间会随 canonical 会话消息写入 PostgreSQL 或 Redis，并在下一轮恢复到模型历史；Ark SDK 因而会自动发送 `previous_response_id` 与未缓存的增量上下文。缓存过期或历史中没有可用 response ID 时，服务会自动回退为完整历史请求，不影响会话正确性。

会话消息接口会触发实际模型推理，并允许 Agent 选择已注册的 MCP 工具；默认运行时使用 DeepAgent 的标准模型—工具循环，单轮最多进行 12 次迭代。`/ping` 接口只用于服务存活检查。

当前允许暴露给 Agent 的远程 MCP 工具为：

| 工具                   | 作用                                                     |
| ---------------------- | -------------------------------------------------------- |
| `tongji.student.score` | 将请求内校园 access token 转交远程 MCP，查询指定学期成绩 |

每次 Tool 调用会从当前请求 context 读取格式正确的 Bearer access token，并以 `X-Tongji-Access-Token` 注入远程 MCP 请求；缺失 token 时 Tool 在本地返回未授权提示，不会发起 MCP 请求。远程 MCP 与同济开放平台仍必须验证 token 的有效性、用户绑定和 scope；部署远程 MCP 时必须保护该请求头，不能写入普通日志。

## 路由一览

| 方法   | 路径                                 | 用途                          |
| ------ | ------------------------------------ | ----------------------------- |
| `GET`  | `/v1/ping`                           | 健康检查                      |
| `POST` | `/v1/sessions`                       | 创建认证或匿名会话            |
| `POST` | `/v1/sessions/:session_id/messages`  | 以 SSE 执行并保存一轮会话消息 |
| `GET`  | `/v1/sessions/:session_id/messages`  | 读取会话历史                  |
| `GET`  | `/v1/sessions/:session_id/task-plan` | 读取活动任务计划              |

## 说明

- A2A 架构和原字节内部 MCP Client 已移除。
- Hertz 已替换为开源 `github.com/cloudwego/hertz`。
