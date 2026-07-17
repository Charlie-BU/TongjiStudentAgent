# Fornax Trace 深度分析

对 AI Agent 的分布式追踪链路进行结构化诊断，包括调用树可视化、性能瓶颈识别、错误根因定位和优化建议。

## 适用场景（何时调用）

- 用户提到 trace 分析、trace 诊断、span 分析、链路分析、Agent 运行轨迹分析
- 需要对 `fornax-cli trace get` 的结果进行深入分析
- trace ID / log-id 排查
- 定位 Agent 性能瓶颈或工具调用错误

## 前置依赖

- `fornax-cli`：用于获取 trace 数据。鉴权与配置详见 fornax-cli SKILL.md "配置与鉴权" 部分
- Python 3：运行分析脚本
- 分析脚本位于：`scripts/trace_analysis/`（analyze_trace.py, dissect_span.py, trace_common.py）

## 分析工作流

### Step 1: 获取 Trace 数据

> trace/span 命令的完整参数详见 `references/commands/trace.md`

```bash
# 按 trace-id 获取（SSO 方式）
fornax-cli trace get --trace-id <TRACE_ID> --workspace-id <SPACE_ID> --last-n-minutes 10080 --format json -o ./traces

# 按 log-id 获取
fornax-cli trace get --log-id <LOG_ID> --workspace-id <SPACE_ID> --last-n-minutes 10080 --format json -o ./traces

# AK/SK 方式
fornax-cli trace get --trace-id <TRACE_ID> --ak <AK> --sk <SK> --last-n-minutes 10080 --format json -o ./traces
```

输出文件：`./traces/trace_<TRACE_ID>.json`

⚠️ **时间窗口参数极其重要**：默认时间窗口很小，旧 trace 会返回空结果。建议用 `--last-n-minutes` 或 `--since/--until` 确保覆盖目标时间。

### Step 2: 全局视图（先全局后局部）

运行分析脚本获取 span 树 + 统计概览：

```bash
python3 scripts/trace_analysis/analyze_trace.py <trace_file> --mode full
```

输出包含：
- Span 树结构（含耗时、模型名、token 数、错误标记）
- 统计摘要：总耗时、错误数、token 总量
- Model 分解：每个模型的调用次数、总耗时、平均耗时、耗时占比、token 数
- Tool 分解：每个工具的调用次数、耗时、成功率
- Agent 分解：每类 Agent 的实例数、总/平均/最大耗时和错误数

### Step 3: 下钻分析

根据全局视图发现的问题，选择性下钻：

```bash
# 单 span 深度拆解（AI 友好的结构化 JSON 报告）
python3 scripts/trace_analysis/dissect_span.py <trace_file> --span-id <SPAN_ID> [--max-content-len 5000]

# 按需选择 section（减少输出）
python3 scripts/trace_analysis/dissect_span.py <trace_file> --span-id <SPAN_ID> --sections identity,anomalies

# 因果链追踪：找到触发某个 tool 的上游 model
python3 scripts/trace_analysis/analyze_trace.py <trace_file> --mode upstream --span-id <TOOL_SPAN_ID>

# 子树统计：只统计某个 span 及其后代的指标
python3 scripts/trace_analysis/analyze_trace.py <trace_file> --mode stats --span-id <SPAN_ID> --json

# 错误链路分析：自动识别根因 + 级联分组
python3 scripts/trace_analysis/analyze_trace.py <trace_file> --mode errors
```

### Step 4: 输出分析结论

结论必须：
- 引用具体 span 数据（格式：`[Span:$Name($SpanID)]`）
- 只陈述可验证的事实，不猜测
- 给出可操作的优化建议

---

## 分析方法论

### Span 数据结构

每个 span 的关键字段：

| Field | Type | Description |
|-------|------|-------------|
| span_id | string | Unique span identifier |
| parent_id | string | Parent span ID (builds the tree) |
| span_name | string | Human-readable name (e.g. "ChatModel", "execute") |
| span_type | string | Type: model, tool, ToolsNode, graph, Chain, Agent, Lambda |
| duration | string | Duration in milliseconds (note: string type) |
| status | string | "success" or "error" |
| status_code | int | 0=success, -1=error |
| input | string | JSON string of input data |
| output | string | JSON string of output data |
| started_at | string | Start timestamp in milliseconds |
| custom_tags | object | Tags: model_name, input_tokens, output_tokens, tokens, agent_name, etc. |
| system_tags | object | Infrastructure: cluster, env, pod_name, region, etc. |

### 典型 Span 层级（Eino ReAct Agent）

```
Agent (top-level)
 └─ Chain
     └─ graph (ReAct loop)
         ├─ Init (Lambda)
         ├─ ChatModel (model) ← LLM call #1
         ├─ ToolNode (ToolsNode) ← tool execution wrapper
         │   ├─ execute (tool)  ← actual tool call
         │   └─ execute (tool)
         ├─ ChatModel (model) ← LLM call #2 (after tool results)
         ├─ ToolNode (ToolsNode)
         │   └─ skill (tool)
         ├─ ChatModel (model) ← LLM call #3
         └─ Lambda (final output)
```

### 错误分类

| Pattern | Likely Cause | Investigation |
|---------|-------------|---------------|
| Model status=error, duration<500ms | Rate limiting or API error | Check if multiple model calls fail in sequence |
| Model status=error, duration>5000ms | Timeout | Check model_name and token counts |
| Tool status=error | Tool execution failure | Read tool input/output, then check upstream model |
| Sequential model errors | Retry loop hitting rate limit | Check for repeated patterns with similar timestamps |
| ToolCall has input but no output | Tool crash or report gap | Check upstream model's output for the tool_call params |

### 性能瓶颈识别

- **Model time dominance**: 若 model 调用占总耗时 >80%:
  - 减少 input_tokens（裁剪 context/history）
  - 简单子任务使用更快模型
  - 并行化独立的 model 调用
- **Token explosion**: 高 input_tokens 通常表明：
  - 累积消息历史未截断
  - 过于冗长的 system prompt
  - Tool 输出被完整回传
- **Tool latency**: 若 tool 调用慢：
  - 检查外部 API 是否瓶颈
  - 检查是否有冗余/重复的 tool 调用

### ReAct Loop 分析

1. 每次迭代 = Model → ToolNode → Model
2. 通过计算 model 调用次数判断迭代轮数
3. 迭代越多 = agent 越难完成任务
4. 检查后续迭代是否重复相同 tool 调用（潜在死循环）

### Custom Tags 参考

| Tag | Present On | Description |
|-----|-----------|-------------|
| model_name | model spans | LLM model identifier |
| input_tokens | model spans | Input token count |
| output_tokens | model spans | Output token count |
| tokens | model spans | Total tokens |
| latency_first_resp | model spans | Time to first token (streaming) |
| agent_name | graph/Agent spans | Agent identifier |
| stream | model spans | Whether streaming was used |
| call_options | model spans | JSON of model call config |
| reasoning_tokens | model spans | Tokens used for chain-of-thought |

---

## 脚本参考

### analyze_trace.py — 全局分析

| Mode | 用途 | 需要 --span-id |
|------|------|---------------|
| `full` | 树 + 统计（默认） | 可选（指定则只输出子树/子树统计） |
| `tree` | 只输出 span 树 | 可选（指定则只输出子树） |
| `stats` | 只输出统计（加 `--json` 输出 JSON） | 可选 |
| `detail` | 某个 span 的原始 JSON 详情 | 必须 |
| `upstream` | 查找 tool/ToolsNode 的上游 model | 必须 |
| `errors` | 自动错误链路分析（根因识别 + 级联分组） | 否 |

额外参数：

| 参数 | 用途 | 示例 |
|------|------|------|
| `--top N` | 输出 Top N 最慢 span 排行（stats/full 模式） | `--top 10` |
| `--top-type TYPE` | 过滤 --top 的 span 类型 | `--top-type model` |

> `--span-id` 支持前缀匹配：传入 ID 前缀即可自动匹配完整 ID。

`errors` 模式自动分析全部错误 span，输出：
- **根因识别**：在祖先链中没有其他错误 span 的错误 span 被标记为根因
- **级联分组**：每个根因下的所有后代错误 span 归为一组，按级联数量降序排列
- **祖先路径**：展示根因 span 在 trace 中的上下文位置

### dissect_span.py — 单 span 深度拆解

用法：`python3 scripts/trace_analysis/dissect_span.py <trace_file> --span-id <SPAN_ID> [--max-content-len N] [--sections SECTIONS]`

> `--span-id` 支持前缀匹配。
>
> `--sections` 按需选取报告 section（逗号分隔），默认输出全部。可用值：`identity, performance, input, output, timeline, upstream, children, anomalies, tags`。

输出结构化 JSON 报告，包含 9 个分析维度：

| Section | 内容 | 适用 span 类型 |
|---------|------|---------------|
| `1_identity` | 身份信息（名称、类型、状态、耗时、模型名） | 全部 |
| `2_performance` | Token 统计、首 token 延迟、call_options | model |
| `3_input_breakdown` | 输入结构化拆解 | 全部 |
| `4_output_breakdown` | 输出结构化拆解 | 全部 |
| `5_timeline_context` | 时间线上下文（前后各 5 个同级 span） | 全部 |
| `6_upstream_model` | 触发当前 tool 的上游 model 信息 | tool, ToolsNode |
| `7_children` | 子 span 列表 | 有子节点时 |
| `8_anomalies` | 自动异常检测标记 | 有异常时 |
| `9_other_tags` / `9_environment` | 关键标签和环境信息 | 全部 |

---

## 核心分析原则

1. **先全局后局部**：必须先看树和统计，再下钻具体 span
2. **因果链追踪**：分析 tool 问题时，必须同时查看触发该 tool 的上游 model
3. **错误连锁分析**：连续的 model 错误通常意味着 API 限流或重试循环
4. **时间单位**：duration 字段单位为 ms
5. **ToolCall 无输出**：如果 tool span 只有 input 没有 output，可能是工具崩溃或上报缺失
6. **Span 引用格式**：`[Span:$Name($SpanID)]`
