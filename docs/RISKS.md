# 当前风险

本文记录当前版本公开部署前仍需处理的风险。

## P0：错误开启本地 Sandbox 会授予宿主机文件与命令执行权限

`POST /v1/agent/chat` 可选接收 HTTP `Authorization: Bearer <access_token>`，并仅将格式正确的 token 放入本次请求的私有上下文。当前阶段不会因 token 缺失或格式错误拒绝调用，也尚未验证 token 有效性、绑定可信用户身份或检查 scope；接入远程 MCP 前必须补齐可信网关或同济开放平台的 token 验证。

当前 `SANDBOX_ENABLED` 默认关闭，未设置或为 `false` 时不会注册本地文件系统或 Shell middleware。若在公开部署环境错误设为 `true`，聊天服务会装配本地文件系统 middleware，并将本地 backend 同时作为 `StreamingShell`：

- 可读取、写入和编辑本机文件；
- 可遍历和搜索文件；
- 可通过 `/bin/sh -c` 执行命令；
- `local.Config{}` 未配置命令校验规则，也未限制可访问的根目录。

因此，恶意输入或提示注入可能诱导模型读取 `.env` 等敏感文件、修改部署环境中的文件，或在服务进程权限范围内执行命令。公开部署必须保持该开关关闭，且接口不得直接暴露到不受信任网络。

提交和部署前应至少完成以下一项：移除文件系统/Shell middleware；改用隔离沙箱并限制工作目录；或为文件访问和命令执行配置严格的路径与命令白名单。同时，接口必须放在认证、授权和访问控制之后。

涉及位置：`biz/handler/agent.go`、`internal/application/chat/service.go`、`internal/integration/sandbox/local.go`。

## 已缓解：普通日志记录用户内容、响应内容和敏感请求头

HTTP 日志现在仅记录生成的 Request ID、方法、路径、状态码和耗时；Chat 服务也不再记录完整 Agent 回复。`Authorization`、Cookie、JWT、请求体、响应体和 Tool Result 不应进入普通日志。

后续如需诊断采样，必须另建受限安全日志通道，并实施字段级脱敏、显式开关和访问审计。
