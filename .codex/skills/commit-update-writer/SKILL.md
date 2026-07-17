---
name: "commit-update-writer"
description: "基于用户指定的改动范围（默认当前工作区）分析上下游依赖并产出改动说明，按时间顺序增量写入 docs/CHANGELOG.md 最上方。用户要求“生成本次改动文档/更新记录/changelog”时触发。"
---

# Commit Update Writer

用于对“用户指定范围”的改动做完整理解，并将结果以项目既有文风写入 `docs/CHANGELOG.md`。  
该 skill 的目标不是罗列 diff，而是解释**改动目的、业务逻辑、实现结果、上下游影响与风险**。

## 触发时机

- 用户要求“生成本次改动说明 / 更新日志 / 变更文档 / changelog”
- 用户要求“基于 git diff 写 markdown 到 docs/CHANGELOG.md”
- 用户要求“分析改动及其依赖、被依赖影响并沉淀文档”

## 范围判定（强制）

1. 默认范围是 `working tree`（未提交改动），不自动混入 `HEAD~1..HEAD`。
2. 仅当用户明确提到“上次提交 / HEAD~1 / 最近一次 commit / 已提交改动”时，才使用 `HEAD~1..HEAD`。
3. 仅当用户明确要求“包含已提交 + 未提交”时，才使用混合范围：`HEAD~1..HEAD` + `working tree`。
4. 若用户说“本次改动/本次变动/当前改动”且工作区同时存在已提交与未提交差异，默认解释为 `working tree`；若语义不清，必须先向用户确认范围再写文档。

## 强制输入约定

- `base_commit` 默认：`HEAD~1`（仅用于记录 `Base Commit` 与需要时的提交范围对比）
- `compare_scope` 必填：`working_tree_only` | `last_commit_only` | `last_commit_plus_working_tree`

## 强制执行规则

1. 每次必须在文档中显式写出基线 commit id（`Base Commit`）。
2. 每次必须读取并遵循风格参考：  
   `.trae/skills/commit-update-writer/reference/language-style.md`
3. 输出文件固定为：`docs/CHANGELOG.md`（注意拼写）。
4. 只能**把新记录增量插入到文件最上方**，禁止修改/删除已有历史记录内容。
5. 若 `docs/CHANGELOG.md` 不存在，先创建，再写入首条记录。
6. 每次必须显式写出“当前内容撰写时间”，格式固定为 `YYYY-MM-DD HH:mm`（24 小时制）。
7. 每次输出区块末尾必须给出一条建议 `git commit message`，遵循 git-cz 规范（`type(scope): subject`），语义准确且长度适中（建议不超过 72 个字符）。
8. 生成正文前，必须先在草稿里写明“本次采用的 compare scope”；若与用户指令不一致，必须停止并重新确认，禁止继续写入。

## 分析范围（强制）

不仅要读 diff 文件，还要覆盖以下上下文：

- 改动代码本身（变更 hunks）
- 改动部分直接依赖的上游（imports、被调用函数/类、配置来源）
- 依赖改动部分的下游（调用方、消费方、命令入口、graph 节点链路）
- 相关业务链路（CLI、channel、graph、service、database、prompt）

## 执行步骤

1. 判定范围
    - 先根据“范围判定（强制）”确定 `compare_scope`。
    - 若存在歧义，先问用户，不得猜测。
2. 锁定基线
    - 解析并记录 `HEAD~1` 对应 commit 的完整/短 SHA（作为 `Base Commit`）。
3. 收集变更
    - 按 `compare_scope` 获取文件级和 hunk 级 diff。
    - 标注新增、修改、删除、重命名。
4. 构建上下游依赖图
    - 对每个改动点定位：上游依赖、下游调用、运行入口。
    - 识别是否存在“改动点未同步其依赖方”的断链风险。
5. 还原业务逻辑
    - 改动前行为是什么。
    - 改动动机是什么（要解决什么问题）。
    - 改动后行为和结果是什么。
6. 评估影响与风险
    - 功能正确性、兼容性、稳定性、性能、可维护性。
    - 明确“已验证项 / 未验证项 / 建议补充验证”。
7. 按风格生成文档片段并写入
    - 生成一个“本次改动记录”区块。
    - 插入到 `docs/CHANGELOG.md` 最上方，保持整体按时间从新到旧排列。
8. 生成建议 commit message
    - 根据本次改动主线，产出 1 条 git-cz 风格 message：`type(scope): subject`。
    - `subject` 需简洁，不写空泛词，不堆叠多主题。

## 顶部增量写入模板（必须包含）

```markdown
## CHANGELOG - <YYYY-MM-DD HH:mm> - <一句话主题>

### 撰写时间

- <YYYY-MM-DD HH:mm>

### Base Commit

- <commit_id>

### Compare Scope

- <working_tree_only | last_commit_only | last_commit_plus_working_tree>

### 背景与改动目标

- <为什么改>

### 改动概览

- <改了哪些模块/文件，核心动作是什么>

### 关键链路解析（含上下游）

- 上游依赖：<依赖了什么，是否变更>
- 当前改动：<核心实现与逻辑>
- 下游影响：<谁依赖它，是否需要同步改>

### 改动结果与业务影响

- <结果、收益、边界、代价>

### 风险与待办

- <潜在风险>
- <建议补充验证 / 后续动作>

### 建议 Commit Message（git-cz）

- `<type>(<scope>): <subject>`
```

## 质量标准

- 文档必须能回答：
    - 改动是否合理
    - 是否可能引入新问题
    - 是否可能导致上下游依赖链路异常
- 不得只贴 diff，不得只列文件名。
- 结论必须有代码依据，不写无依据猜测。

## 禁止事项

- 禁止覆盖、改写或重排单条历史记录的正文内容。
- 禁止写入 `docs/UPDATE.md`（文件名错误）或其他非目标文件（除非用户明确改口）。
- 禁止省略 `Base Commit`（其值应来自 `HEAD~1`）。
- 禁止省略 `Compare Scope`。
- 禁止省略“撰写时间”（`YYYY-MM-DD HH:mm`）。
- 禁止输出不符合 git-cz 的 commit message，或过长、语义不清的 subject。
- 禁止忽略依赖与被依赖影响分析。
- 禁止在用户未授权时自动把“已提交改动”与“未提交改动”混写为同一条记录。
