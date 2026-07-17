---
name: "commit-quality-reviewer"
description: "针对当前仓库执行指定改动范围的差异质检（默认当前工作区）。触发于用户要求审查本次改动、检查代码变更、review 代码、代码质检等场景。"
---

# Commit Diff Quality Reviewer

用于对 **用户指定范围** 的改动做质量审查（默认 `working tree`）。  
该 skill 只审查 diff 覆盖内容，不对未变更代码做泛化点评。

## 最重要质检目标（最高优先级）

所有审查结论必须优先回答以下 3 个问题：

1. 当前改动是否合理
    - 是否真正解决目标问题，是否存在过度设计/错误抽象/不必要复杂度。
2. 是否引入新的问题
    - 是否带来新 bug、安全隐患、性能退化、异常处理退化或可维护性显著下降。
3. 是否破坏依赖改动部分的代码可用性
    - 是否导致调用方、上下游模块、配置、脚本、测试或运行链路无法正常工作。

## 触发时机

- 用户要求"对比上次提交做质检 / review / CR"
- 用户要求"审查本次改动"、"检查本次变更"、"review 本次改动"
- 用户要求"代码质检"、"审查代码"、"检查代码质量"
- 用户提供 commit id，要求"当前版本 vs 某次 commit"审查
- 提交 commit / 发起 MR 前的质量闸门检查
- 用户要求对当前工作区或暂存区改动做审查

## 输入约定（强制）

- `base_commit`：
    - 默认：`HEAD~1`（当前版本对比上次提交）
    - 指定：用户显式给出 commit id（如 `abc1234`）
- `compare_scope`：
    - `working_tree_only`：仅审查未提交改动（默认）
    - `last_commit_only`：仅审查 `HEAD~1..HEAD`
    - `last_commit_plus_working_tree`：审查 `HEAD~1..HEAD` + 未提交改动
- `whitelist`：读取 `.trae/skills/commit-quality-reviewer/docs/whitelist.md`

若用户要求“某次 commit 对比”，但未给 commit id，必须先追问并等待确认。
若用户仅说“本次改动/当前改动”且未明确范围，默认使用 `working_tree_only`。

## 审查范围与边界

- 仅基于 `compare_scope` 对应的 diff 范围进行检查
- 仅报告高置信度真实问题（默认置信度 >80%）
- 不报告纯风格偏好、不报告未变更代码的历史问题
- 遇到白名单豁免项时，标记为 `WAIVED`，不计入阻塞结论

## 通用代码审查基线（保留）

以下为跨项目通用标准，与项目定制清单并行生效；若同一问题同时命中，按更高严重级别处理。

### CRITICAL（通用）

- 敏感信息泄露：密钥、密码、token、隐私数据出现在代码、日志或错误信息
- 注入类风险：SQL 注入、命令注入、模板/脚本注入、反序列化风险
- 认证/授权绕过：越权读取、越权修改、权限校验缺失
- 高风险破坏性操作：无保护的数据删除、不可回滚的结构变更、关键资源误操作

### HIGH（通用）

- 功能正确性回归：边界条件遗漏、空值路径崩溃、错误分支返回不一致
- 异常处理缺陷：吞异常、空 `except/catch`、错误被静默忽略
- 并发与资源问题：竞态条件、连接/句柄泄露、未关闭任务或会话
- 向后兼容破坏：公共接口参数语义变化且无迁移/兼容策略

### MEDIUM（通用）

- 性能退化：明显多余循环、重复 I/O、无缓存热点查询、N+1 模式
- 观测性不足：关键失败路径缺日志、日志缺上下文难定位
- 测试覆盖不足：高风险改动缺最小回归验证

### LOW（通用）

- 可维护性问题：重复代码、命名歧义、过长函数、魔法值散落
- 代码卫生问题：临时调试语句、无效注释代码、陈旧 TODO

## 执行步骤

1. 确认基线 commit
    - 先判定 `compare_scope`；若语义不清，先向用户确认范围，不得猜测。
2. 确认基线 commit
    - 默认 `HEAD~1`；若用户给 commit id，则使用用户给定值。
3. 收集 diff 上下文
    - 获取变更文件列表与关键 hunks。
    - 对每个变更点，补读所在文件必要上下文（导入、调用链、异常处理、类型定义）。
4. 先做“三问主检”（最高优先）
    - 合理性检查：改动动机、实现路径、复杂度与收益是否匹配。
    - 新问题检查：是否引入新的功能/稳定性/安全/性能风险。
    - 依赖影响检查：改动接口、数据结构、配置或行为后，依赖方是否仍可正常运行。
5. 读取白名单
    - 加载 `.trae/skills/commit-quality-reviewer/docs/whitelist.md`。
    - 命中白名单的条目，按规则降级为 `WAIVED`。
6. 按通用 + 项目清单审查
    - 先看 `CRITICAL/HIGH`，再看 `MEDIUM/LOW`。
7. 输出结论
    - 先列问题，再给摘要与裁决。
    - 对每个问题给出“证据 + 风险 + 建议修复”。

## 收尾建议规则（强制）

- 当审查场景包含 `last_commit_only`（即 `base_commit = HEAD~1`）时，任务结束后必须附带如下建议：
    - 建议用户执行 `commit-update-writer`，为本次改动生成并追加更新文档到 `docs/CHANGELOG.md`。
- 该规则仅为“建议”，禁止在 `commit-quality-reviewer` 中自动触发或代执行 `commit-update-writer`。

## 项目定制审查清单（Immortality）

在“通用代码审查基线”之外，额外检查以下项目特有风险。

### CRITICAL（必须阻塞）

- 密钥/凭据泄露：`API Key`、数据库 URI、Lark 凭据、token 被硬编码或打印到日志
- 权限与数据越权：`user_id/fr_id` 相关查询未做 ownership 校验即读写
- Prompt/消息注入风险：将未清洗外部输入直接拼接系统提示词并影响关键决策
- 破坏数据库安全：迁移脚本误删/误改核心表或 enum，导致不可逆数据损坏

### HIGH（合并前应修复）

- Graph 并发与单例被破坏：重复构建 graph/client，缺失锁保护，导致并发初始化风险
- Graph state 更新语义错误：该做 patch 却整表覆盖，导致短期记忆/日志丢失
- 异步链路阻塞：在 async 路径引入明显阻塞 I/O 或重计算，影响服务稳定性
- CLI 退出码与错误处理退化：吞错、误报成功、`KeyboardInterrupt`/业务异常处理不一致
- 数据库连接管理问题：engine/session 生命周期不当，可能泄露连接或跨进程复用失效

### MEDIUM（建议修复）

- 可观测性不足：关键流程（graph 节点、DB 写入、外部调用）缺少必要日志或错误上下文
- 发布链路隐患：版本号、tag、构建步骤与 `publish.yml` 约定不一致
- 规则/枚举不一致：`FineGrainedFeedDimension`、状态字段、prompt 环境变量命名漂移
- 测试缺口：改动触达 graph/CRUD/CLI 但无对应回归验证

### LOW（可跟进）

- 临时调试痕迹：`print/pprint`、注释掉的旧逻辑、一次性调试分支
- 文档与实现轻微不一致：README 描述滞后、参数默认值未同步
- 可维护性信号：重复逻辑、命名歧义、魔法值过多

## 白名单（豁免）机制

- 文件：`.trae/skills/commit-quality-reviewer/docs/whitelist.md`
- 允许豁免的典型场景：
    - 临时保留的 `print` 调试输出
    - 计划短期保留的注释代码
    - 已知技术债但有明确截止时间与负责人
- 豁免条目至少包含：
    - 匹配范围（文件路径/关键字/正则）
    - 豁免原因
    - 失效时间（建议）
    - 负责人（建议）

## 输出格式（强制）

先给 Findings，再给 Summary。按严重级别从高到低排序。

```markdown
## Core Check

- Reasonableness: PASS | WARN | FAIL
- New Issues Introduced: NO | YES
- Dependency Impact: SAFE | RISKY | BROKEN

## Findings

- [CRITICAL] <问题标题>
    - File: <path>
    - Evidence: <触发代码或行为>
    - Risk: <具体风险>
    - Fix: <可执行修复建议>
    - Status: OPEN | WAIVED

## Summary

| Severity | Count(Open) | Count(Waived) |
| -------- | ----------: | ------------: |
| CRITICAL |           0 |             0 |
| HIGH     |           1 |             0 |
| MEDIUM   |           2 |             1 |
| LOW      |           1 |             0 |

Decision: BLOCK | WARN | PASS
Base Commit: <base_commit>
Compared Scope: <working_tree_only | last_commit_only | last_commit_plus_working_tree>
Compared Range: <scope对应的真实范围>
```

## 裁决标准

- `BLOCK`：存在任意 `CRITICAL`（且非 WAIVED）
- `WARN`：无 CRITICAL，但存在 `HIGH`（且非 WAIVED）
- `PASS`：无 `CRITICAL/HIGH` 的 OPEN 问题

## 禁止事项

- 禁止跳过基线确认，直接做“全仓库扫描式”点评
- 禁止把白名单当成永久忽略机制（过期应恢复检查）
- 禁止报告无证据推断或低置信度猜测
- 禁止把“未变更历史问题”伪装成本次 diff 问题
- 禁止在用户未授权时自动将“已提交改动”与“未提交改动”混合作为审查范围
