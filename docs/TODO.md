# TODO

## P0：上线与主链路完整性

- **补充动态上下文压缩摘要**：保留原始 canonical 消息，记录摘要覆盖的 sequence；为 tool result、reasoning 与超长单条消息定义截断和重建策略。
- **验证 Ark Responses API response-chain 缓存**：在隔离 endpoint 观测 `previous_response_id`、缓存命中率、动态 reminder 顺序与过期后的完整历史回退。
- **补齐会话整轮一致性与重试策略**：为未完成 Run 增加可识别状态或提交标记，明确客户端重试幂等键，避免部分工具轨迹被作为完整历史回放。
- **收敛 OAuth 与请求身份边界**：将用户 ID enrichment 移到请求校验之后；为 OAuth `state` 增加短期一次性存储和浏览器关联，并明确 token scope、失效、撤销与匿名会话的绑定策略。
- **完成 MCP Tool 治理**：为每个工具定义风险等级、身份/Scope 前置校验、参数 Schema、超时、结果大小上限、错误分类和数据脱敏边界；固定远程 Tool catalog 快照。
- **完善知识库 Tool 元数据与运营闭环**：`system.search_knowledge` 已作为显式只读 Tool 接入，并对检索失败进行可控降级；仍需让上游条目稳定提供适用范围、来源 URL 与更新时间。
- **封装同济同学 1.0 Tool**：先选定少量高频、只读、可验证的校园场景，完成 MCP Schema、上游错误归一、授权边界与端到端联调后再扩展目录。
- **SSE 可靠性**：补充心跳、显式取消、断线后状态查询/重连策略，以及模型、MCP、知识库调用的超时与取消传播测试。
- **生产验证**：在预发布环境执行 PostgreSQL 迁移与 Redis 会话 round-trip；确认启动依赖、数据库权限、缓存 TTL、锁续租失败的 fail-closed 策略和告警。

## P1：能力完善与运营闭环

- **Skills 撰写**：先沉淀少量高频、边界清晰的校园场景；维护 manifest、allowlist、版本和测试，避免把通用知识无条件塞入系统提示词。
- **系统内置 Tools 补充**：确定计划能力采用 DeepAgent 内置 Todo 还是受控的 `system.manage_task_plan`；补齐任务状态、事件协议与多步骤失败恢复，再考虑专用子 Agent。
- **知识库运营闭环**：建立来源审核、适用人群/校区标注、核验时间、失效下线、未命中问题与用户反馈的录入流程。
- **观测与隐私**：复核 `assistant.reasoning`、工具 arguments/result 的前端会话授权、存储保留期和日志脱敏；如需诊断采样，建立受限审计通道。
- **评测与 CI**：建立高频校园问答、工具选择、越权拒绝、缓存回退和会话恢复的离线用例；接入覆盖率、`go test -race`、`go vet` 与迁移检查门禁。
- **HITL / Checkpoint**：仅在引入写操作或高风险能力时实现持久化 Checkpoint、确认事件和 Resume；首期只读工具不因此阻塞。
