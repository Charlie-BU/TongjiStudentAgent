# TongjiStudent

TongjiStudent 是一个面向同济大学校园场景的 Agent 服务基架。项目使用开源 [CloudWeGo Hertz](https://www.cloudwego.io/zh/docs/hertz/) 提供 HTTP 服务，使用 [CloudWeGo Eino](https://github.com/cloudwego/eino) 构建 DeepAgent，并内置一个可替换的 MCP Server 示例。

目前项目已完成基础运行时、模型配置、DeepAgent 和 MCP 工具初始化；校园业务能力（如课表、成绩、班车查询）及对话 HTTP 接口仍待落地。

## 当前能力

- 开源 Hertz HTTP 服务，默认监听 `8080` 端口。
- 健康检查接口：`GET /`、`GET /ping`、`GET /v1/ping`。
- Agent 调用接口：`POST /v1/agent/chat`。
- 基于 Ark 兼容配置初始化 Eino DeepAgent。
- 进程内 MCP Server 示例，注册了 `get_current_time` 工具。
- 可选 Fornax 评测/Trace 集成，默认关闭。
- 本地日志模块，不直接使用项目业务代码中的字节内部日志库。

## 项目结构

```text
.
├── biz/handler/           # HTTP 处理函数与健康检查接口
├── internal/
│   ├── application/chat/  # /v1/agent/chat 应用服务与依赖装配
│   ├── agentic/runtime/   # 与具体模型、工具解耦的 DeepAgent 运行时封装
│   ├── integration/       # Ark、知识库、Fornax、MCP 与本地 Sandbox 适配
│   └── platform/          # 服务配置与日志等基础能力
├── script/                # 构建产物启动脚本
├── .env                   # 本地配置（已被 Git 忽略）
├── main.go                # Hertz 服务入口
├── router.go              # 自定义路由
└── router_gen.go          # Hertz 路由注册
```

## 前置条件

- Go `1.23.8`（项目的 `go.mod` 指定的 toolchain 版本）。
- 可访问项目 Go 依赖。虽然 Fornax 默认不运行，但其隔离模块仍在同一 Go module 内；首次下载依赖时，如本机没有缓存，仍需要具备访问相关内部依赖的权限。
- 可用的模型 Endpoint 凭据。

## 配置本地环境

在项目根目录创建或编辑 `.env`。该文件不会提交到 Git。

```env
# Ark/模型服务配置：ARK_BASE_URL 优先于 ARK_BASE_URL_CN
ARK_BASE_URL_CN=https://your-model-endpoint
ENDPOINT_ID=your-endpoint-id
ENDPOINT_API_KEY=your-api-key

# 可选的内部 Fornax 评测与 Trace 集成；本地默认关闭
FORNAX_ENABLED=false
# FORNAX_AK=your-fornax-ak
# FORNAX_SK=your-fornax-sk

# Ark 知识库检索：启用后，检索结果会作为参考资料注入主 Agent 调用链
ARK_KNOWLEDGE_ENABLED=false
# ARK_AK=your-knowledge-ak
# ARK_SK=your-knowledge-sk
# ARK_KNOWLEDGE_COLLECTION=your-collection-name
# ARK_KNOWLEDGE_PROJECT=default
# ARK_KNOWLEDGE_RESOURCE_ID=your-resource-id # 可替代 COLLECTION
# ARK_KNOWLEDGE_LIMIT=5
# ARK_KNOWLEDGE_DOMAIN=api-knowledgebase.mlp.cn-beijing.volces.com

# 本地文件系统与 Shell 工具；默认关闭，仅限受控本地开发环境
SANDBOX_ENABLED=false
```

`ENDPOINT_ID`、`ENDPOINT_API_KEY` 以及 `ARK_BASE_URL`（或 `ARK_BASE_URL_CN`）均为必填项。服务启动时会检查它们是否存在并据此创建模型客户端。

如需启用 Fornax，请显式设置：

```env
FORNAX_ENABLED=true
FORNAX_AK=your-fornax-ak
FORNAX_SK=your-fornax-sk
```

启用但未同时提供 `FORNAX_AK` 和 `FORNAX_SK` 时，服务会以明确错误退出。

启用知识库时，必须配置 `ARK_AK`、`ARK_SK`，以及
`ARK_KNOWLEDGE_COLLECTION` 或 `ARK_KNOWLEDGE_RESOURCE_ID`。主 Agent 会先检索知识库，再将命中的内容作为非可信参考资料传入同一次模型调用；不会再启动独立的知识库模型调用链。

`SANDBOX_ENABLED` 未设置或为 `false` 时，Agent 不会注册文件系统或 Shell middleware。设为 `true` 会让 Agent 使用本机 Backend 执行文件操作和命令，仅可用于受控本地开发环境，禁止在公开部署环境开启。

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

运行测试以验证本地 MCP Server 及其他已覆盖模块：

```bash
go test ./...
```

## 调用 Agent

服务启动后，可用以下请求调用 DeepAgent：

```bash
curl --request POST http://127.0.0.1:8080/v1/agent/chat \
  --header 'Content-Type: application/json' \
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

`message` 为空或请求体不是合法 JSON 时会返回 `400`；模型调用或 Agent 执行失败时会返回 `500`。当前接口为非流式单轮调用，不保存会话历史。

## 模型与 MCP 的现状

启动过程中会依次创建模型客户端、进程内 MCP Client、MCP 工具和 DeepAgent。因此，服务能成功启动代表模型配置格式和 MCP 初始化已通过。

`POST /v1/agent/chat` 会触发实际模型推理，并允许 Agent 选择已注册的 MCP 工具；`/ping` 系列接口只用于服务存活检查。当前接口为单轮非流式接口，后续可在此基础上加入会话持久化和 SSE 流式输出。

内置 MCP demo 的工具为：

| 工具 | 作用 |
| --- | --- |
| `get_current_time` | 返回当前 UTC 时间（RFC3339 格式） |

## 路由一览

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/` | 基础服务响应 |
| `GET` | `/ping` | 健康检查 |
| `GET` | `/v1/ping` | 兼容部署平台的存活检查 |
| `POST` | `/v1/agent/chat` | 单轮调用 DeepAgent |

## 说明

- A2A 架构和原字节内部 MCP Client 已移除。
- Hertz 已替换为开源 `github.com/cloudwego/hertz`。
- Fornax 相关内部依赖仅由 `integration/fornax` 隔离层引用；要让根依赖树完全不包含它们，需要进一步将该集成拆分为独立进程或独立仓库模块。
