# TongjiStudent A2A 接入模块

独立的入站协议适配层，直接调用 `internal/application/chat.Service`；不复制 Agent、模型选择、检索或工具执行逻辑。原有 `/v1/sessions` HTTP/SSE 接口保持可用。

使用官方 `github.com/a2aproject/a2a-go/v2` SDK，提供 A2A 1.0 JSON-RPC，以及官方 `a2acompat/a2av0` 的 0.3 兼容接口。协议参考：[A2A specification](https://a2a-protocol.org/latest/specification/)、[官方 Go SDK](https://github.com/a2aproject/a2a-go)。

## 启用与地址

A2A 路由随现有 Hertz 服务默认注册，共用监听端口，无需环境变量或独立监听服务。

每次请求 Agent Card 时，使用请求的 HTTP/HTTPS 协议与 Host（含端口）生成 `/a2a/v1` 和 `/a2a/v0.3` 地址，不缓存其他请求的地址。支持 HTTP 和 HTTPS。

当前大陆服务器的地址如下：

| 用途 | 部署后对外地址 |
| --- | --- |
| Agent Card | `http://124.223.93.75:8080/a2a/.well-known/agent-card.json` |
| 旧发现路径别名 | `http://124.223.93.75:8080/a2a/.well-known/agent.json` |
| A2A 1.0 JSON-RPC | `http://124.223.93.75:8080/a2a/v1` |
| A2A 0.3 JSON-RPC | `http://124.223.93.75:8080/a2a/v0.3` |

反向代理应保留外部 Host，并原样转发 `/a2a/*` 路径。当前不会自动信任 `Forwarded` / `X-Forwarded-*` 头，也不会推断被代理剥离的 `/api` 前缀；TLS 终止在代理时需由受信任的服务入口规范化请求协议。

Agent Card 无需认证，包含两个版本的接口和 `streaming: true`。版本 0.3 客户端读取卡片的 `url`；1.0 客户端读取 `supportedInterfaces`。

流式响应是向 JSON-RPC 地址 **POST** `SendStreamingMessage`（1.0）或 `message/stream`（0.3）后获得的 `text/event-stream`，不是向某个 `/sse` 地址发 GET。平台表单名为“SSE URL”不代表它一定实现这个协议；需要平台实际解析和联调确认。

## 身份和会话

所有 RPC 都要求 `Authorization: Bearer <当前用户的校园 OAuth access token>`，复用校园用户信息查询验证身份。无效凭据返回 HTTP 401，不降级为匿名；`SYS_USERID` 或请求中自填的用户标识不能替代 token。不要在平台共享配置中填入个人 token。

首轮省略 `contextId`，服务生成新的 A2A context 和后端 session。后续轮次携带响应中的 `contextId`、新的 `messageId`；已完成的任务不要再次作为新请求的 `taskId`。context 映射和任务读写按已验证的用户身份隔离。未知或其他用户的 context 拒绝接入。

当前适配器使用现有默认 `lite` 模型层，支持文本输入。文件与结构化 Data Part 明确返回不支持，不会悄悄丢弃。Agent 内部仍可执行现有工具。公开输出包含回答和状态，不包含原始推理、工具参数/结果或上游异常详情。

## 流式生命周期

- `assistant.delta` 实时映射为同一 artifact 的文本增量；后续增量使用 `append: true`。
- 最后一个 artifact 更新标记 `lastChunk: true`，之后发布 completed 状态。
- 后端失败发布 failed；主动取消中断运行上下文并发布 canceled。
- 支持 SDK 的任务读取、取消及订阅语义，不宣称支持 push notification。
- 最长执行 5 分钟；SDK 无活动超时 2 分钟；请求体 64 KiB，单次文本输出 1 MiB。
- Hertz 通过官方 net/http adaptor 转发并 Flush。响应设置 `X-Accel-Buffering: no`。部署代理也必须关闭缓冲，并允许足够长的读超时。

## 当前存储边界

A2A task 与 context→session 映射保存在**单进程内存**中，分别最多 1000 条；满额后拒绝新任务/会话，重启清空。后端已认证会话仍由原有存储管理，但本模块尚不能跨重启恢复 A2A 映射。该版本适用于单实例联调；正式多实例和持续运行前，需要实现持久化 task store、context 映射以及保留/清理策略，不能仅依赖负载均衡。

## 本地验证

```sh
go test ./internal/transport/a2a
go test -race ./internal/transport/a2a
go test ./...
```

测试使用模拟 ChatService，不调用付费模型或真实校园工具。通过真实 HTTP 连接验证两个协议版本的首段增量、最终状态、多轮 session 复用、跨用户隔离；另外经过实际 Hertz 路由验证 Flush、任务取消及不支持内容的拒绝。

## 同济低码平台联调

代码添加不代表线上服务已部署，也不代表 TongjiStudent2.0 已接入。上一轮平台检查发现访问部署域名的代理连接超时，见 `docs/TONGJI_PLATFORM_INTEGRATION_2026-09-23.md`。

部署并打通平台到服务的网络后，通过 Computer Use 在平台创建 A2A 服务，确认其协议版本和 Agent Card 发现方式，再配置调用节点与逐用户授权。验收必须包括：真实增量显示、多轮会话、取消、错误、两个用户不串数据。若平台只能提供固定共享凭据，需要先补齐身份授权桥接，不能共享个人校园 token。
