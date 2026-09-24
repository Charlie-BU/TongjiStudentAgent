# TongjiStudent2.0 平台接入核查（2026-09-23）

状态：尚未完成接入和发布；端到端流式输出未验证。所有检查均通过本机 Computer Use 完成。

## 当前服务接口

核对来源：README.md 与 router.go（13、17 行等）。部署基址由用户提供：https://tongjistudent.charliebu.cn/api

- GET /v1/ping：无请求体、无需鉴权的健康检查。
- POST /v1/sessions：创建会话，返回 session_id。匿名会话可不带 Authorization。
- POST /v1/sessions/:session_id/messages：唯一 Agent 执行入口，请求体包含 message，可省略 model_tier（默认 lite）。
- 消息响应为自定义 text/event-stream：assistant.delta 的 text 为回答增量；另有 run.started、run.completed、run.failed、工具事件和任务计划事件。
- 认证会话使用校园 OAuth Bearer token；平台 SYS_USERID 不能直接替代 token，也不能将单个用户 token 共享给所有用户。

## 平台实测

在 TongjiStudent2.0 中临时添加 HTTP 节点，采用 GET、超时 15 秒、重试 0 次单节点调试。

| 目标 | 结果 | 耗时 |
| --- | --- | --- |
| https://tongjistudent.charliebu.cn/api/v1/ping | ProxyError / Cannot connect to proxy / TimeoutError | 15041 ms |
| https://agent.tongji.edu.cn/ | HTTP 200 | 108 ms |

目标请求完整错误（未包含凭据）：

```text
ProxyError: HTTPSConnectionPool(host='tongjistudent.charliebu.cn', port=443): Max retries exceeded with url: /api/v1/ping (Caused by ProxyError('Cannot connect to proxy.', TimeoutError('timed out')))
```

结论：平台 HTTP 节点本身可工作，但平台出口到部署域名的链路失败。尚不能据此确定是域名出口策略、DNS、目标防火墙还是网络路由问题，需要从平台实际运行环境检查。该请求无 JSON 请求体、无鉴权，因此改写业务请求参数不能解决本次失败。

## 流式接入能力

- 已检查 HTTP 节点界面及平台 HTTP 文档：输出为 status_code、headers、body、files；未发现将上游 SSE 增量直接转发给对话界面的配置。
- 工作空间存在 A2A 服务入口，创建表单要求 SSE URL 并自动解析。
- 现有自定义会话 SSE 不能仅因采用 SSE 就视为兼容 A2A；已检查的 router.go 未见 A2A 对外路由。
- A2A 是候选接入路线，需取得平台实际支持的协议版本、发现方式、事件格式、鉴权方式，并验证对话流节点是否增量透传。
- 等完整 HTTP body 再输出不满足用户要求的真正流式输出。

## 后续实施与验收

1. 从平台代理实际运行环境打通 tongjistudent.charliebu.cn:443，确认 GET /api/v1/ping 返回 HTTP 200。
2. 确认平台 A2A 接入契约；据此添加并部署协议适配层，复用现有 Agent 执行服务，不重复实现 Agent 逻辑。
3. 建立平台会话与后端 session_id 的隔离映射，保留多轮上下文；设计逐用户授权绑定，不信任客户端随意提交的用户 ID。
4. 将 assistant.delta、完成与失败事件转换为平台支持的流式协议；保留取消、超时与错误语义，避免自动重试重复执行有副作用的任务。
5. 在 TongjiStudent2.0 配置并验证：首个文本增量先于完成事件到达、多轮记忆、两个用户不串会话、匿名能力与授权后校园能力、异常与取消。全部通过后再发布。

平台 HTTP 文档：https://365.kdocs.cn/wiki/l/0lcmTTM2kDctAv
平台应用：https://agent.tongji.edu.cn/product/llm/workspace/cramcl81pcfvmmbds030/application/dappc2f710969i0rrkeg/arrange?tabKey=arrange
