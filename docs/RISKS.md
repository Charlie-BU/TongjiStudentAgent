# 当前风险

本文记录当前版本公开部署前仍需处理的风险。

## 已知豁免：写入 access token context 时同步查询用户基础信息

`platformauth.WithAccessToken` 在写入格式正确的校园 access token 后，会同步调用同济开放平台的用户基础信息接口以补充 `user_id`。该函数由 `biz/handler/agent.go` 在 JSON 请求体校验之前调用，因此无效请求也可能访问上游；同时 MCP、测试及未来其他调用方使用该 context setter 时，也会隐式触发该查询。

当前产品决定接受该行为，并按 `WL-20260731-001` 临时豁免至 2026-08-30。上游调用失败时，请求 context 仅保留 access token，不阻断现有聊天调用。

到期前必须复核并至少落实其中一种收敛方式：先完成请求体校验再补充用户 ID；将身份查询移至显式应用层步骤；或拆分无副作用的 token setter 与用户 ID enrichment API。任何新增调用方都不得假设 `WithAccessToken` 为纯 context 写入函数。

## 已缓解：普通日志记录用户内容、响应内容和敏感请求头

HTTP 日志现在仅记录生成的 Request ID、方法、路径、状态码和耗时；Chat 服务也不再记录完整 Agent 回复。`Authorization`、Cookie、JWT、请求体、响应体和 Tool Result 不应进入普通日志。

后续如需诊断采样，必须另建受限安全日志通道，并实施字段级脱敏、显式开关和访问审计。
