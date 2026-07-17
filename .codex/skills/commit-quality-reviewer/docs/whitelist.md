# Whitelist（Commit Diff Review 豁免清单）

用于登记“已知但暂时允许”的审查问题。  
该文件可随时更新，审查时命中条目会标记为 `WAIVED`。

## 使用原则

- 仅豁免短期可解释问题，不豁免安全红线
- 每条尽量设置 `expires_at`，避免永久失效
- 代码已修复后应及时删除对应条目

## 条目模板

```yaml
- id: WL-20260505-001
  enabled: true
  severity: LOW
  type: debug_print
  match:
    file: src/agents/graphs/FRBuildingGraph.py
    contains: "pprint.pprint("
  reason: "本地图调试阶段暂留，待 Graph 日志改造后删除"
  owner: "your_name"
  created_at: "2026-05-05"
  expires_at: "2026-05-20"
```

字段说明：

- `enabled`：是否生效（`true/false`）
- `severity`：预期被豁免的问题级别（`CRITICAL/HIGH/MEDIUM/LOW`）
- `type`：问题类型（如 `debug_print`、`commented_code`、`known_debt`）
- `match.file`：命中的文件路径（相对仓库根目录）
- `match.contains`：命中的关键字（或可扩展为正则）
- `reason`：豁免原因
- `owner`：责任人
- `expires_at`：建议失效时间（到期应复核）

## 示例（按需保留/修改）

```yaml
- id: WL-EXAMPLE-001
  enabled: false
  severity: LOW
  type: commented_code
  match:
    file: src/cli/commands/fr.py
    contains: "# TODO: remove legacy flow"
  reason: "等待与旧命令兼容窗口结束后清理"
  owner: "team"
  created_at: "2026-05-05"
  expires_at: "2026-06-01"
```
