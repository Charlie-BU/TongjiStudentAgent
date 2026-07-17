# fornax-cli experiment

Experiment 资源相关命令，包含：提交实验、查询实验列表/详情、拉取结果与聚合结果。

## experiment list

按筛选条件查询实验列表，按 `updated_at` 倒序返回。

### 用法

```bash
fornax-cli experiment list [选项]
```

### 筛选

- `--name`：实验名模糊搜索。
- 结构化筛选（多个 flag 之间用 AND 组合；同一个 flag 重复传多次表示"或"）：
  - `--status`（可重复）：按实验状态筛选。可选值：
    - `pending`、`processing`、`success`、`failed`、`terminated`、`system_terminated`、`terminating`、`draining`
  - `--target-type`（可重复）：按评测对象类型筛选。可选值：
    - 基础类型：`coze_bot`、`coze_loop_prompt`、`trace`、`coze_workflow`、`volcengine_agent`、`custom_rpc_server`
    - 扩展类型：`volcengine_agent_agentkit`、`web_agent`、`a2a_agent`、`custom_agent`
    - Online 变体（仅用于展示，不参与评测执行）：上述各类型对应的 `*_online` 后缀，如 `coze_bot_online`、`coze_loop_prompt_online`、`coze_workflow_online`、`volcengine_agent_online`、`custom_rpc_server_online`、`volcengine_agent_agentkit_online`
  - `--expt-type`（可重复）：按实验类型筛选。可选值：`offline`、`online`
  - `--eval-set-id` / `--evaluator-id` / `--target-id` / `--creator`：按对应资源 ID 或创建人精确匹配。
- `--raw-filter-option`：JSON 对象，对应服务端 `filter_option` 字段，用于上述 flag 无法表达的高级筛选场景。与上述结构化筛选 flag **互斥**。

### 分页

- `--page-no`（默认 1）/ `--page-size`（默认 20，1–200）
- `--limit`：跨页累计最多返回 N 条（`0` 表示只取单页；当 `N > page-size` 时自动翻页凑满 N）

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_list_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
# 基本用法
fornax-cli experiment list

# 名称模糊搜索 + 分页
fornax-cli experiment list --name demo --page-size 20

# 多状态 + target_type
fornax-cli experiment list --status pending --status processing --target-type coze_loop_prompt

# 按 eval_set / 创建人
fornax-cli experiment list --eval-set-id <EVAL_SET_ID> --creator <USER_ID>

# 高级：raw filter_option（用于结构化 flag 无法表达的查询条件）
fornax-cli experiment list --raw-filter-option '{"fuzzy_name":"demo","filters":{"filter_conditions":[{"field":{"field_type":"expt_status"},"operator":"in","value":"2,3"}],"logic_op":"and"}}'

# 写入 -o
fornax-cli experiment list -o ./out
```

### 关注字段

list 接口返回的 experiment 比 detail 接口字段更少，pretty 表格展示：

- `id` / `name` / `status`
- `expt_template_meta.expt_type`（如 `offline`，可能为空）
- `expt_stats.success_turn_count` / `expt_stats.failed_turn_count`
- `started_at` / `ended_at`

> 对应 OpenAPI：`POST /open-api/evaluation/v1/experiments/list`（`ListExperimentsOApi`）。

## experiment submit

提交一个新的评测实验。

### 用法

```bash
fornax-cli experiment submit [选项]
```

### 必填输入

- `--eval-set-id` / `--eval-set-version`
- `--evaluator`（可重复传入，格式：`<id>:<version>`）或 `--target-type` / `--target-id`
- 跳过评测对象：`--skip-target`（此时不允许传 target 相关参数与 target 映射）
- 跳过评估器：`--skip-evaluator`（此时不允许传 evaluator 相关参数与 evaluator 映射）

### 字段映射

- 简单映射：`--target-map` / `--evaluator-map-from-evalset` / `--evaluator-map-from-target`
- 或直接传 raw JSON：`--raw-target-field-mapping` / `--raw-evaluator-field-mapping`

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_<experiment_id>.json`

### 示例

#### 基本用法

```bash
fornax-cli experiment submit \
  --name demo \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version 1.0.0 \
  --evaluator <ID:VERSION> \
  --target-type coze_loop_prompt \
  --target-id <TARGET_ID> \
  -o ./out
```

#### coze\_loop\_prompt 完整示例（最常用）

```bash
fornax-cli experiment submit \
  --name "<EXPERIMENT_NAME>" \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version "<EVAL_SET_VERSION>" \
  --evaluator "<PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>" \
  --evaluator "<CODE_EVAL_ID>:<CODE_EVAL_VERSION>" \
  --target-type coze_loop_prompt \
  --target-id <PROMPT_ID> \
  --target-version "<PROMPT_VERSION>" \
  --target-map query=input \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:input=input \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:expected_output=expected_output \
  --evaluator-map-from-target  <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:actual_output=actual_output \
  --evaluator-map-from-evalset <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:expected_output=expected_output \
  --evaluator-map-from-target  <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:actual_output=actual_output \
  --item-retry-num 3 \
  --format json
```

字段映射说明：

- `--target-map query=input`：把 eval-set 的 `input` 字段映射到 target 的 `query` 输入字段
- `--evaluator-map-from-evalset`：把 eval-set 字段映射到 evaluator 输入字段
- `--evaluator-map-from-target`：把 target 输出字段映射到 evaluator 输入字段（如 `actual_output`）

#### 跳过评测对象（--skip-target）

适用：eval-set 已包含 evaluator 需要的所有输入字段，不需要执行 target。

```bash
fornax-cli experiment submit \
  --name "<EXPERIMENT_NAME>" \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version "<EVAL_SET_VERSION>" \
  --evaluator "<PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>" \
  --skip-target \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:user_input=case_id \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:agent_output=case_url \
  --format json
```

注意：`--skip-target` 模式下不允许传 `--target-type/--target-id/--target-version/--target-map/--raw-target-field-mapping`，也不允许使用 `--evaluator-map-from-target`。

#### 跳过评估器（--skip-evaluator）

适用：仅执行 target 产出输出，后续再用 evaluator 评分。

```bash
fornax-cli experiment submit \
  --name "<EXPERIMENT_NAME>" \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version "<EVAL_SET_VERSION>" \
  --skip-evaluator \
  --target-type custom_rpc_server \
  --target-id <TARGET_ID> \
  --target-version "<TARGET_VERSION>" \
  --env ppe_fornax_eval \
  --target-runtime-param-json '{"model_name":"glm5.0"}' \
  --format json
```

注意：`--skip-evaluator` 模式下不允许传 `--evaluator` 与任何 evaluator mapping（`--evaluator-map-*` / `--raw-evaluator-field-mapping`）。

#### 带 target 映射的简单示例

```bash
fornax-cli experiment submit \
  --name demo \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version 1.0.0 \
  --evaluator <ID:VERSION> \
  --target-type coze_loop_prompt \
  --target-id <TARGET_ID> \
  --target-map output=golden \
  -o ./out
```

### 参数说明

#### 必填

- `--name <name>`：实验名称（必填）
- `--eval-set-id <EVAL_SET_ID>`：评测集 id（必填）
- `--eval-set-version <ver>`：评测集版本（必填），示例：`1.0.0`
- `--evaluator <id>:<version>`：参与评测的 evaluator（除非设置 `--skip-evaluator`，否则必填；可重复）
  - 格式：`<id>:<version>`
  - 示例：`--evaluator 123:1.0.0 --evaluator 456:1.0.1`
- `--target-type <type>`：target 类型（除非设置 `--skip-target`，否则必填）。示例：`prompt`、`coze_loop_prompt`、`coze_bot`、`coze_workflow`、`custom_rpc_server`（以服务端支持为准）
- `--target-id <id>`：target id（除非设置 `--skip-target`，否则必填）
- `--skip-target`：跳过评测对象，仅基于 eval-set 的字段驱动 evaluator（不允许使用 target 相关参数与 target 映射）
- `--skip-evaluator`：跳过评估器，仅执行 target（不允许使用 evaluator 相关参数与 evaluator 映射）

#### 可选：实验描述、并发与重试

- `--description <text>`：实验描述（可选）
- `--concurrency <N>`：并发数（可选）；`0` 表示使用服务端默认值
- `--item-retry-num <N>`：单条样本重试次数（可选）；`0` 表示使用服务端默认值
- `--bot-info-type <type>`：bot info 类型（可选），可选值：`draft_bot`、`product_bot`
- `--bot-publish-version <ver>`：Bot 发布版本（可选），针对发布类型目标

#### 可选：target 版本与区域

- `--target-version <ver>`：target 版本（可选）
- `--target-region <region>`：target 区域（默认 `cn`），可选值：`boe`、`cn`、`i18n`
- `--env <env>`：target 运行环境（可选），例如 `ppe_fornax_eval`（不支持与 `--skip-target` 同时使用）
- `--custom-eval-target-json '<json_object>'`：自定义 eval target 配置（可选，JSON object），支持字段：`id`、`name`、`avatar_url`、`ext`（不支持与 `--skip-target` 同时使用）
- `--target-runtime-param-json '<json_object>'`：target 运行时参数（可选，JSON object 字符串），例如注入模型/环境变量（不支持与 `--skip-target` 同时使用）

#### 字段映射（两种方式：简单映射或 raw JSON）

##### 方式 1：简单映射（可重复）

- `--target-map <target_field>=<eval_set_field>`：把 eval-set 字段映射到 target 字段
  - 示例：`--target-map output=golden`
- `--evaluator-map-from-evalset <evaluator_id>:<version>:<field>=<evalset_field>`：把 eval-set 字段映射到 evaluator 输入字段
- `--evaluator-map-from-target <evaluator_id>:<version>:<field>=<output_field>`：把 target 输出字段映射到 evaluator 输入字段

上述三个参数都是 stringArray，可重复传多次以表达多条映射关系。

##### 方式 2：raw JSON（高级用法，可选）

适合映射关系复杂或不便用“key=value”表达时使用。建议用单引号包裹 JSON，避免 shell 转义。

- `--raw-target-field-mapping '<json_object>'`：target 字段映射（JSON object）
- `--raw-evaluator-field-mapping '<json_array>'`：evaluator 字段映射（JSON array）

#### 其他

- `-h, --help`：显示本子命令帮助信息并退出

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `experiment_<experiment_id>.json`

## experiment detail

获取实验详情。

### 用法

```bash
fornax-cli experiment detail [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_<experiment_id>.json`

### 示例

```bash
fornax-cli experiment detail --experiment-id <EXP_ID>
fornax-cli experiment detail --experiment-id <EXP_ID> --api open-api -o ./out
fornax-cli experiment detail --experiment-id <EXP_ID> --format json
```

### 关注字段

- `experiment.status`：实验状态，可能的值：`pending` / `processing` / `success` / `failed` / `terminated`
- `experiment.expt_stats`：各状态样本计数
  - `pending_turn_count` / `processing_turn_count` / `success_turn_count` / `failed_turn_count` / `terminated_turn_count`
- `experiment.target_field_mapping` / `experiment.evaluator_field_mapping`：字段映射配置，可用于下一次 `experiment submit` 直接复用

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）
- `--api <api>`：选择查询的 API 路径（默认 `open-api`），可选值：`open-api`、`loop`

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `experiment_<experiment_id>.json`

## experiment results

分页获取实验结果。

### 用法

```bash
fornax-cli experiment results [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_results_<experiment_id>.json`

### 示例

```bash
fornax-cli experiment results --experiment-id <EXP_ID>
fornax-cli experiment results --experiment-id <EXP_ID> --page-no 1 --page-size 20 -o ./out
fornax-cli experiment results --experiment-id <EXP_ID> --limit 100 --format pretty
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）
- `--limit <N>`：最多返回 N 条；`0` 表示不限制
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `experiment_results_<experiment_id>.json`

## experiment agg-results

获取实验聚合结果。

### 用法

```bash
fornax-cli experiment agg-results [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_agg_results_<experiment_id>.json`

### 示例

```bash
fornax-cli experiment agg-results --experiment-id <EXP_ID>
fornax-cli experiment agg-results --experiment-id <EXP_ID> -o ./out
fornax-cli experiment agg-results --experiment-id <EXP_ID> --format json
```

### 返回数据说明

聚合结果主要包含两类数据：

- `evaluator_results`：每个 evaluator 的聚合分数（`average / sum / max / min / distribution`）
- `eval_target_aggr_result`：目标侧性能/成本聚合（`latency(ms)`、`input_tokens / output_tokens / total_tokens`，同样有 `average/sum/max/min/distribution`）

### CI 准出用法

#### 效果准出：平均分低于阈值则失败

```bash
fornax-cli experiment agg-results \
  --experiment-id <EXPERIMENT_ID> \
  --format json \
| python3 - <<'PY'
import json,sys

MIN_SCORE = 0.9
data = json.load(sys.stdin)

bad = []
for ev in data.get("evaluator_results", []):
    avg = None
    for ar in ev.get("aggregator_results", []):
        if ar.get("aggregator_type") == "average":
            avg = (ar.get("data") or {}).get("value")
            break
    if avg is None:
        bad.append((ev.get("name") or ev.get("evaluator_id"), "missing-average"))
        continue
    if float(avg) < MIN_SCORE:
        bad.append((ev.get("name") or ev.get("evaluator_id"), float(avg)))

if bad:
    print(f"QUALITY_GATE_FAIL min_score={MIN_SCORE} bad={bad}")
    sys.exit(1)
print(f"QUALITY_GATE_PASS min_score={MIN_SCORE}")
PY
```

#### 性能/成本准出：基于 latency/tokens 做阈值判断

```bash
fornax-cli experiment agg-results \
  --experiment-id <EXPERIMENT_ID> \
  --format json \
| python3 - <<'PY'
import json,sys

MAX_AVG_LATENCY_MS = 2500
MAX_AVG_TOTAL_TOKENS = 600

data = json.load(sys.stdin)
target = data.get("eval_target_aggr_result", {})

def avg_of(metric_name: str):
    for ar in target.get(metric_name, []) or []:
        if ar.get("aggregator_type") == "average":
            return (ar.get("data") or {}).get("value")
    return None

avg_latency = avg_of("latency")
avg_total_tokens = avg_of("total_tokens")

bad = []
if avg_latency is None or float(avg_latency) > MAX_AVG_LATENCY_MS:
    bad.append(("avg_latency_ms", avg_latency, MAX_AVG_LATENCY_MS))
if avg_total_tokens is None or float(avg_total_tokens) > MAX_AVG_TOTAL_TOKENS:
    bad.append(("avg_total_tokens", avg_total_tokens, MAX_AVG_TOTAL_TOKENS))

if bad:
    print(f"PERF_COST_GATE_FAIL bad={bad}")
    sys.exit(1)
print(f"PERF_COST_GATE_PASS avg_latency_ms={avg_latency} avg_total_tokens={avg_total_tokens}")
PY
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `experiment_agg_results_<experiment_id>.json`

## experiment retry

重试已完成或失败的评测实验。

### 用法

```bash
fornax-cli experiment retry [选项]
```

### 重试模式

- `retry_failure`（默认）：仅重试失败的样本
- `retry_all`：重试全部样本
- `retry_target_items`：重试指定的样本（需要 `--item-id`）

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_retry_<experiment_id>.json`

### 示例

```bash
# 重试失败样本（默认模式）
fornax-cli experiment retry --experiment-id <EXP_ID>

# 重试全部样本
fornax-cli experiment retry --experiment-id <EXP_ID> --retry-mode retry_all

# 重试指定样本
fornax-cli experiment retry --experiment-id <EXP_ID> --retry-mode retry_target_items --item-id 123 --item-id 456

# 输出到文件
fornax-cli experiment retry --experiment-id <EXP_ID> --format json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）
- `--retry-mode <mode>`：重试模式（可选，默认 `retry_failure`），可选值：`retry_all`、`retry_failure`、`retry_target_items`
- `--item-id <ID>`：需要重试的样本 ID（可重复传入），当 `--retry-mode` 为 `retry_target_items` 时必填

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `experiment_retry_<experiment_id>.json`

### 返回数据

- `run_id`：新的运行 ID

## experiment export

提交一个实验报告导出任务，返回 `export_id`，后续用 `experiment export-record` 查询任务状态与下载链接。

> 对应 OpenAPI：`POST /v1/loop/evaluation/experiments/:experiment_id/results/export`（`ExportExperimentResultOApi`）。

### 用法

```bash
fornax-cli experiment export [选项]
```

### 必填输入

- `--experiment-id <EXP_ID>`：实验 id
- `--export-type <type>`：导出类型，可选值：`CSV`（当前仅支持 CSV）

### 列规格（`export_columns`）

按需勾选要导出的列，至少需要传一项；可重复或用逗号分隔。

- `--eval-set-fields <key>[,<key>...]`：评测集字段 key 列表（对应 `ColumnEvalSetField.Key`）
- `--eval-target-outputs <name>[,<name>...]`：评测对象输出字段名（如 `actual_output`、`trajectory` 以及自定义输出名）
- `--metrics <name>[,<name>...]`：性能指标列名，例如 `eval_target_total_latency`、`eval_target_input_tokens`、`eval_target_output_tokens`、`eval_target_total_tokens`
- `--evaluator-version-ids <id>[,<id>...]`：评估器版本 id 列表；每个 id 会导出对应的 `score` 与 `reason` 两列
- `--weighted-score`：导出加权总分列（依赖实验上各 evaluator 的 `score_weight` 配置，体现在 detail 响应的 `evaluator_id_version_list[*].score_weight`）
- `--tag-key-ids <id>[,<id>...]`：人工标注列的 `TagKeyID` 列表
- `--export-columns-file <file>` 或 `--raw-export-columns '<json>'`：直接以 JSON 形式传入完整的 `ExptResultExportColumnSpec`（高级用法；与上面简写参数互斥）

`ExptResultExportColumnSpec` JSON 结构示例：

```json
{
  "eval_set_fields": ["input", "expected_output"],
  "eval_target_outputs": ["actual_output"],
  "metrics": ["eval_target_total_latency", "eval_target_total_tokens"],
  "evaluator_version_ids": [123, 456],
  "weighted_score": true,
  "tag_key_ids": [9001]
}
```

### 示例

#### 基本用法（简写参数）

```bash
fornax-cli experiment export \
  --experiment-id <EXP_ID> \
  --export-type CSV \
  --eval-set-fields input,expected_output \
  --eval-target-outputs actual_output \
  --metrics eval_target_total_latency,eval_target_total_tokens \
  --evaluator-version-ids 123,456 \
  --weighted-score \
  -o ./out
```

#### 通过 JSON 文件传完整 column spec

```bash
fornax-cli experiment export \
  --experiment-id <EXP_ID> \
  --export-type CSV \
  --export-columns-file ./columns.json \
  --format json
```

### 输出文件

- `-o <DIR>`：写入 `experiment_export_<experiment_id>.json`
- 不传 `-o`：直接输出到标准输出（STDOUT）

返回数据包含：

- `data.export_id`：导出任务 id，用于后续 `experiment export-record` 查询

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）
- `--export-type <type>`：导出类型，目前可选值：`CSV`（必填）
- 列规格相关参数见上文「列规格」小节
- 列规格至少需要选一项；`--export-columns-file` / `--raw-export-columns` 与简写参数互斥

## experiment export-record

查询某次导出任务的状态、错误信息与下载链接。

> 对应 OpenAPI：`GET /v1/loop/evaluation/experiments/:experiment_id/export_records/:export_id`（`GetExperimentResultExportRecordOApi`）。

### 用法

```bash
fornax-cli experiment export-record [选项]
```

### 必填输入

- `--experiment-id <EXP_ID>`：实验 id
- `--export-id <EXPORT_ID>`：`experiment export` 返回的导出任务 id

### 示例

```bash
fornax-cli experiment export-record \
  --experiment-id <EXP_ID> \
  --export-id <EXPORT_ID> \
  --format json

# 落盘 record JSON
fornax-cli experiment export-record \
  --experiment-id <EXP_ID> \
  --export-id <EXPORT_ID> \
  -o ./out

# 一步下载 CSV（推荐）：传目录，文件名自动用 expt_<EXP_ID>_<EXPORT_ID>.csv
fornax-cli experiment export-record \
  --experiment-id <EXP_ID> \
  --export-id <EXPORT_ID> \
  --download ./out

# 也可以指定完整文件名
fornax-cli experiment export-record \
  --experiment-id <EXP_ID> \
  --export-id <EXPORT_ID> \
  --download ./report.csv
```

### 关注字段

返回的 `data.expt_result_export_record`：

- `csv_export_status`：导出任务状态，可能值：`Unknown` / `Running` / `Success` / `Failed`
- `start_time` / `end_time`：开始与结束时间戳（毫秒）
- `expired`：下载链接是否已过期
- `url`：成功状态下的 CSV 下载链接；过期或失败时不可用
- `error`：失败时的错误信息（`code` / `message` / `detail`）

### `--download` 行为

- 值是已存在的目录 → 文件保存为 `<dir>/expt_<EXP_ID>_<EXPORT_ID>.csv`
- 值是文件路径 → 直接写入该路径（父目录必须存在）
- 自动对 URL path 里的非 ASCII 字符（如中文文件名）做百分号编码——CDN 的签名校验对此敏感，未编码会被 TOS 拒（`secure-time-check-md5-failed` / 403）
- 非 `Success`、`expired=true` 或 `url` 为空时，**`--download` 失败并以非 0 退出码退出**；record JSON 仍会正常打印/落盘
- 与 `-o` 独立：`-o` 控制 record JSON 落盘，`--download` 控制 CSV 落盘，可同时使用

### 轮询示例（等待导出完成）

```bash
EXP_ID=<EXP_ID>
EXPORT_ID=<EXPORT_ID>

while :; do
  resp=$(fornax-cli experiment export-record \
    --experiment-id "$EXP_ID" --export-id "$EXPORT_ID" --format json)
  status=$(echo "$resp" | jq -r '.expt_result_export_record.csv_export_status')
  echo "status=$status"
  case "$status" in
    Success)
      # 进入终态后用 --download 一步拿 CSV
      fornax-cli experiment export-record \
        --experiment-id "$EXP_ID" --export-id "$EXPORT_ID" \
        --download "./expt_${EXP_ID}.csv"
      break;;
    Failed)
      echo "export failed:"; echo "$resp" | jq '.expt_result_export_record.error'
      exit 1;;
  esac
  sleep 5
done
```

### 输出文件

- `-o <DIR>`：写入 `experiment_export_record_<experiment_id>_<export_id>.json`
- `--download <PATH>`：见上文「`--download` 行为」
- 不传 `-o` 也不传 `--download`：record JSON 输出到 STDOUT

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--experiment-id <EXP_ID>`：实验 id（必填）
- `--export-id <EXPORT_ID>`：导出任务 id（必填）
- `--download <PATH>`：状态为 `Success` 时把 CSV 下载到本地，自动处理 URL path 的中文编码；可传目录或文件路径

## 实验生命周期流程

典型流程：提交实验 → 轮询状态 → 拉取聚合结果/明细结果。

### 1) 提交实验

参考上面 `experiment submit` 各种场景的示例。提交后记录输出中的 `experiment.id` 作为 `<EXPERIMENT_ID>`。

### 2) 查看实验状态

```bash
fornax-cli experiment detail --experiment-id <EXPERIMENT_ID> --format json
```

轮询直到 `experiment.status` 进入终态：`success` / `failed` / `terminated`。

### 3) 拉取聚合指标

```bash
fornax-cli experiment agg-results --experiment-id <EXPERIMENT_ID> --format json
```

可配合上面的 CI 准出脚本做效果/性能/成本门禁。

### 4) 拉取明细结果

```bash
fornax-cli experiment results --experiment-id <EXPERIMENT_ID> --limit 100 --format pretty
```

用于定位具体失败样本。

### 5) 导出 CSV 报告（可选）

实验进入终态后，可以异步导出 CSV 报告并直接下载到本地：

```bash
# 提交导出任务（按需选择要导出的列）
EXPORT_ID=$(fornax-cli experiment export \
  --experiment-id <EXPERIMENT_ID> \
  --export-type CSV \
  --eval-set-fields input,expected_output \
  --eval-target-outputs actual_output \
  --metrics eval_target_total_latency,eval_target_total_tokens \
  --evaluator-version-ids <EV_VER_ID_1>,<EV_VER_ID_2> \
  --format json | jq -r '.export_id')

# 查询状态并一步下载（--download 自动处理 URL 中文编码与签名）
fornax-cli experiment export-record \
  --experiment-id <EXPERIMENT_ID> \
  --export-id "$EXPORT_ID" \
  --download ./out
```

详见 `experiment export` / `experiment export-record` 章节。

## 常见排障

- `Invalid value for '--format': 'json~'`：复制命令时混入了多余字符（如末尾 `~`），删掉后重试。
- 字段映射不确定时，先用以下命令查看 eval-set 和 evaluator 的字段定义：
  - `fornax-cli eval-set get --id <EVAL_SET_ID> --format json`
  - `fornax-cli evaluator get --id <EVALUATOR_ID> --format json`
- `--skip-target` 与 target 相关参数互斥；`--skip-evaluator` 与 evaluator 相关参数互斥。
