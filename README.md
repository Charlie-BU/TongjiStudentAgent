# TongjiStudent

TongjiStudent 是一个面向同济大学校园场景的 Agent 服务基架。项目使用开源 [CloudWeGo Hertz](https://www.cloudwego.io/zh/docs/hertz/) 提供 HTTP 服务，使用 [CloudWeGo Eino](https://github.com/cloudwego/eino) 构建 DeepAgent，并内置一个可替换的 MCP Server 示例。

目前项目已完成基础运行时、模型配置、DeepAgent 和 MCP 工具初始化；校园业务能力（如课表、成绩、班车查询）及对话 HTTP 接口仍待落地。

## 当前能力

- 开源 Hertz HTTP 服务，默认监听 `8080` 端口。
- 健康检查接口：`GET /`、`GET /ping`、`GET /v1/ping`。
- Agent 调用接口：兼容 JSON 的 `POST /v1/agent/chat` 与 SSE 的 `POST /v1/agent/chat/stream`，均可选传入短期 Bearer access token。
- 基于 Ark 兼容配置初始化 Eino DeepAgent。
- 启动时连接远程 Streamable HTTP MCP Server，并只向 Agent 暴露 allowlist 中的工具。
- 可选 Cozeloop 集成，用于 Trace 观测与系统 Prompt 管理；可视为开源版 Fornax。
- 本地日志模块，不直接使用项目业务代码中的字节内部日志库。

## 项目结构

```text
.
├── biz/handler/           # HTTP 处理函数与健康检查接口
├── internal/
│   ├── application/chat/  # /v1/agent/chat 应用服务与依赖装配
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
- 可用的模型 Endpoint 凭据。

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

# Ark 知识库检索：启用后，检索结果会作为参考资料注入主 Agent 调用链
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

# 本地文件系统与 Shell 工具；默认关闭，仅限受控本地开发环境
SANDBOX_ENABLED=false

# 同济开放平台 OAuth 2.0 授权码模式
TONGJI_OPEN_PLATFORM_CLIENT_ID=your-client-id
TONGJI_OPEN_PLATFORM_CLIENT_SECRET=your-client-secret
TONGJI_OPEN_PLATFORM_REDIRECT_URI=https://app.tongji.edu.cn/wallbreakerAuth/callback.html
TONGJI_OPEN_PLATFORM_STATE_SECRET=replace-with-a-random-secret
```

`ENDPOINT_ID`、`ENDPOINT_API_KEY` 以及 `ARK_BASE_URL`（或 `ARK_BASE_URL_CN`）均为必填项。服务启动时会检查它们是否存在并据此创建模型客户端。

如需启用 Cozeloop，请显式设置 `COZELOOP_ENABLED=true` 以及对应的 `COZELOOP_*` 变量。当前项目会用它注册 Eino 全局回调，并从 PromptHub 拉取 `prompt.tongjistudent.system_prompt` 作为系统提示词；它承担的是原先 Fornax 对应的观测与 Prompt 管理职责，但这里采用的是开源 Cozeloop 实现。

启用知识库时，必须配置 `ARK_AK`、`ARK_SK`，以及
`ARK_KNOWLEDGE_COLLECTION` 或 `ARK_KNOWLEDGE_RESOURCE_ID`。主 Agent 会先检索知识库，再将命中的内容作为非可信参考资料传入同一次模型调用；不会再启动独立的知识库模型调用链。

`SANDBOX_ENABLED` 未设置或为 `false` 时，Agent 不会注册文件系统或 Shell middleware。设为 `true` 会让 Agent 使用本机 Backend 执行文件操作和命令，仅可用于受控本地开发环境，禁止在公开部署环境开启。

## 同济开放平台浏览器授权

服务提供授权码模式的两个接口，客户端密钥和 state 签名密钥只从 `.env` 读取；`.env` 已被 Git 忽略，可直接参考本 README 中的配置示例。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/v1/tongji/oauth/authorize` | 创建签名 `state`，并 302 跳转到同济统一认证页面。 |
| `POST` | `/v1/tongji/oauth/token` | callback 页面提交 `code` 和 `state`，服务校验 state 后换取并返回短期 Bearer access token。 |

浏览器访问 `https://<你的 Go 服务域名>/v1/tongji/oauth/authorize` 后，开放平台会重定向到已登记的 `https://app.tongji.edu.cn/wallbreakerAuth/callback.html?code=...&state=...`。由于该回调页不在 Go 服务域名下，它必须将参数提交回 Go 服务：

```js
const params = new URLSearchParams(window.location.search);
const response = await fetch("https://<你的 Go 服务域名>/v1/tongji/oauth/token", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ code: params.get("code"), state: params.get("state") }),
});
const bearerToken = await response.json();
```

该接口仅允许 `https://app.tongji.edu.cn` 跨域读取响应，响应包含 `access_token`、`token_type`、`expires_in` 和 `scope`，**不会**把 refresh token 返回给浏览器或写入日志。前端应把 access token 保存在内存中并在过期后重新授权；不得放入 URL、日志或长期 `localStorage`。

本项目尚未实现受信任用户身份和 token 持久化，因此返回的 token 仅用于当前浏览器会话。Chat 接口可由浏览器以 HTTP `Authorization: Bearer <access_token>` 传入该短期 token；服务仅将格式正确的 token 放入本次请求的私有上下文，不写入模型消息、响应或普通日志。当前阶段不会因 token 缺失或格式错误拒绝 Agent 调用，也未完成 token 有效性验证、用户绑定或 scope 审核；这些能力是个人数据 MCP 上线前的前置条件。

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
curl http://127.0.0.1:8080/
curl http://127.0.0.1:8080/ping
curl http://127.0.0.1:8080/v1/ping
```

三个接口都会返回 HTTP `200` 和 JSON 响应，例如：

```json
{"message":"hey yo!"}
```

运行测试以验证远程 MCP 适配及其他已覆盖模块：

```bash
go test ./...
```

## 调用 Agent

服务启动后，可用以下请求调用 DeepAgent：

```bash
curl --request POST http://127.0.0.1:8080/v1/agent/chat \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <access_token>' \
  --data '{"message":"现在几点了？"}'
```

请求体：

```json
{"message":"用户问题"}
```

成功时返回：

```json
{"message":"Agent 的最终文本回复"}
```

`Authorization` 为可选字段，当前仅在 Bearer 格式正确时写入请求上下文；`message` 为空或请求体不是合法 JSON 时会返回 `400`；模型调用或 Agent 执行失败时会返回 `500`。JSON 接口保持单轮聚合回复；SSE 接口同样不保存会话历史。响应会包含用于问题排查的 `X-Request-ID`，普通日志只记录该 ID、方法、路径、状态码和耗时，不记录请求或响应内容。

## 流式调用 Agent

`POST /v1/agent/chat/stream` 使用 Server-Sent Events 返回单轮 Run 的安全可见过程：`run.started`、`agent.status`、`assistant.delta`、`tool.call.started`、`tool.call.completed`、`tool.call.failed`、`run.completed` 与 `run.failed`。每个事件包含同一次运行的 `run_id`、递增 `seq` 和 UTC `occurred_at`。事件不会包含模型原始推理内容、工具参数、工具原始结果或 Bearer token。

```bash
curl --no-buffer --request POST http://127.0.0.1:8080/v1/agent/chat/stream \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <access_token>' \
  --data '{"message":"现在几点了？"}'
```

## 模型与 MCP 的现状

启动过程中会依次创建模型客户端、远程 Streamable HTTP MCP Client、allowlist 中的 MCP 工具和 DeepAgent。因此，服务能成功启动代表模型配置格式、远程 MCP 初始化及允许工具发现均已通过。

`POST /v1/agent/chat` 与 `POST /v1/agent/chat/stream` 都会触发实际模型推理，并允许 Agent 选择已注册的 MCP 工具；`/ping` 系列接口只用于服务存活检查。当前仍是无会话的单轮运行；后续可在此基础上加入 Session、取消与 HITL Resume。

当前允许暴露给 Agent 的远程 MCP 工具为：

| 工具 | 作用 |
| --- | --- |
| `tongji.student.score` | 将请求内校园 access token 转交远程 MCP，查询指定学期成绩 |

每次 Tool 调用会从当前请求 context 读取格式正确的 Bearer access token，并以 `X-Tongji-Access-Token` 注入远程 MCP 请求；缺失 token 时 Tool 在本地返回未授权提示，不会发起 MCP 请求。远程 MCP 与同济开放平台仍必须验证 token 的有效性、用户绑定和 scope；部署远程 MCP 时必须保护该请求头，不能写入普通日志。

## 路由一览

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/` | 基础服务响应 |
| `GET` | `/ping` | 健康检查 |
| `GET` | `/v1/ping` | 兼容部署平台的存活检查 |
| `POST` | `/v1/agent/chat` | 单轮调用 DeepAgent |
| `POST` | `/v1/agent/chat/stream` | 以 SSE 返回单轮 Agent 运行事件 |

## 说明

- A2A 架构和原字节内部 MCP Client 已移除。
- Hertz 已替换为开源 `github.com/cloudwego/hertz`。
