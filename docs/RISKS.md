# 当前风险

本文记录当前版本公开部署前仍需处理的风险。

## P0：错误开启本地 Sandbox 会授予宿主机文件与命令执行权限

`POST /v1/agent/chat` 接口目前没有认证或授权校验，任何能访问服务端口的调用方都可以向 DeepAgent 提交消息。

当前 `SANDBOX_ENABLED` 默认关闭，未设置或为 `false` 时不会注册本地文件系统或 Shell middleware。若在公开部署环境错误设为 `true`，聊天服务会装配本地文件系统 middleware，并将本地 backend 同时作为 `StreamingShell`：

- 可读取、写入和编辑本机文件；
- 可遍历和搜索文件；
- 可通过 `/bin/sh -c` 执行命令；
- `local.Config{}` 未配置命令校验规则，也未限制可访问的根目录。

因此，恶意输入或提示注入可能诱导模型读取 `.env` 等敏感文件、修改部署环境中的文件，或在服务进程权限范围内执行命令。公开部署必须保持该开关关闭，且接口不得直接暴露到不受信任网络。

提交和部署前应至少完成以下一项：移除文件系统/Shell middleware；改用隔离沙箱并限制工作目录；或为文件访问和命令执行配置严格的路径与命令白名单。同时，接口必须放在认证、授权和访问控制之后。

涉及位置：`biz/handler/agent.go`、`internal/application/chat/service.go`、`internal/integration/sandbox/local.go`。

## P1：日志会记录用户内容、响应内容和敏感请求头

请求日志 middleware 会记录完整请求 Header 和 Body，也会记录完整响应 Header 和 Body。`/v1/agent/chat` 的用户问题和 Agent 回复因此会写入日志；若调用方携带 `Authorization`、JWT 或其他凭据，也会一并落盘。

此外，聊天服务会记录完整 Agent 回复。对于校园场景，这些日志可能包含个人信息、成绩、课表或其他敏感业务数据。

提交和部署前应将日志缩减为请求方法、路径、状态码、耗时和请求 ID 等元数据；禁止记录完整 Header、请求体和响应体。确需记录诊断数据时，应使用字段级脱敏、显式采样和受限访问的安全日志通道。

涉及位置：`main.go`、`internal/application/chat/service.go`。
