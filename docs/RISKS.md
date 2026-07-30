# 当前风险

本文记录当前版本公开部署前仍需处理的风险。

## P0：个人数据 MCP 尚未完成可信身份、令牌校验与 scope 审核

`POST /v1/agent/chat` 可选接收 HTTP `Authorization: Bearer <access_token>`，并仅将格式正确的 token 放入本次请求的私有上下文。当前阶段不会因 token 缺失或格式错误拒绝调用，也尚未验证 token 有效性、绑定可信用户身份或检查 scope；接入远程 MCP 前必须补齐可信网关或同济开放平台的 token 验证。

聊天调用链使用 `deep.New`，但不注册宿主机文件系统或 Shell middleware。远程 MCP 与同济开放平台在接受请求级 Bearer token 后，仍必须验证 token 的有效性、绑定可信用户身份并审核工具所需 scope；部署入口也必须位于认证、授权与访问控制之后。

涉及位置：`biz/handler/agent.go`、`internal/application/chat/service.go`、`internal/integration/mcp/`、远程 `TongjiStudentMCPServer`。

## 已缓解：普通日志记录用户内容、响应内容和敏感请求头

HTTP 日志现在仅记录生成的 Request ID、方法、路径、状态码和耗时；Chat 服务也不再记录完整 Agent 回复。`Authorization`、Cookie、JWT、请求体、响应体和 Tool Result 不应进入普通日志。

后续如需诊断采样，必须另建受限安全日志通道，并实施字段级脱敏、显式开关和访问审计。
