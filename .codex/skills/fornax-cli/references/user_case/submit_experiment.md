# Fornax 创建评测实验（端到端）

用于在本仓库内通过 CLI 一次性串起：**创建评测集（Eval Set）→ 写入样本 → 发布版本 → 创建评估器（Evaluator）→ 发布版本 → 提交评测实验（Experiment）→ 拉取结果**。

## 适用场景（何时调用）

- 用户给出 **prompt/bot/workflow 的 ID 与版本**，并要求“从头创建评测集和评估器，然后发起实验”
- 需要把评测集字段与目标输入/输出、评估器输入字段做 **字段映射（field mapping）**
- 需要快速复现实验、排查字段名或 content type 报错

## 你需要从用户/上下文拿到的信息

- `target_type`：通常是 `coze_loop_prompt`（也可能是 `coze_bot/coze_workflow/custom_rpc_server/...`）
- `target_id`：评测对象 ID（例如 prompt\_id）
- `target_version`：评测对象版本（例如 `0.0.2`）
- 评测集 schema 与 items（推荐提供本地 JSON 文件路径）
- 选择的 evaluator：prompt judge / code judge / 两者都要
- region：默认 `cn`（如需 boe/i18n 需显式指定）

## 默认约定（coze\_loop\_prompt）

- target 输入字段：`query`
- target 输出字段：`actual_output`
- 常见映射：`--target-map query=input`

## 操作步骤（推荐按顺序执行）

### 0) 配置鉴权（一次性）

**方式一（推荐）：SSO 登录**

```bash
fornax-cli auth login
fornax-cli config set workspace-id <SPACE_ID>
```

**方式二：AK/SK**

```bash
fornax-cli config set ak <AK>
fornax-cli config set sk <SK>
# 可选：fornax-cli config set endpoint https://fornax.bytedance.net
```

### 1) 查询评测对象（当只有名称，没有 ID 时）

当用户只提供“评测对象类型 + 名称”，你需要先把它解析成 `target_type / target_id / target_version`。

#### 1.1 Prompt：按名称/关键词查 prompt\_id

`prompt list` 支持用 `--keyword` 按名称/前缀检索：

```bash
fornax-cli prompt list \
  --keyword "<PROMPT_NAME_OR_PREFIX>" \
  --page-no 1 \
  --page-size 50 \
  --format json
```

输出里常用字段：

- `id`：用于 `experiment submit --target-id`（也就是 prompt\_id）
- `display_name`：用于和用户给的名称做匹配
- `last_publish_version`：可作为默认 `--target-version`（若用户未指定）

如果 `--keyword` 命中多条，按以下策略挑选：

- `display_name` 精确等于用户给的名称优先
- 仍有多条则要求用户补充更精确的名称，或直接提供 ID

#### 1.2 Prompt：按 prompt\_key 获取详情（可选）

当你已知 `prompt_key`（而不是 id）时：

```bash
fornax-cli prompt get-by-key --key <PROMPT_KEY> --format json
fornax-cli prompt get-by-key --key <PROMPT_KEY> --version <VERSION> --format json
```

> 注意：`prompt get-by-key` 使用旧版 API，Prompt 必须有已发布版本才能通过 key 获取。推荐使用 `prompt list` 查找 prompt\_id 后用 `prompt get-by-id` 获取完整详情。

### 2) 创建评测集 + 写入样本 + 发布版本

```bash
cat > eval_set_schema.json <<'JSON'
{
  "field_schemas": [
    {
      "name": "input",
      "description": "输入文本",
      "content_type": "text",
      "text_schema": "{\"type\": \"string\"}",
      "default_display_format": "plain_text",
      "is_required": true
    },
    {
      "name": "expected_output",
      "description": "期望输出（可选）",
      "content_type": "text",
      "text_schema": "{\"type\": \"string\"}",
      "default_display_format": "plain_text",
      "is_required": false
    }
  ]
}
JSON

cat > eval_set_items.json <<'JSON'
[
  {"input": "Hello", "expected_output": "Hi"},
  {"input": "What is 2+2?", "expected_output": "4"}
]
JSON

fornax-cli eval-set create \
  --name "<eval_set_name>" \
  --schema-file "./eval_set_schema.json" \
  --format json

# 记录输出里的 evaluation_set_id 作为 <EVAL_SET_ID>
fornax-cli eval-set add-items \
  --id <EVAL_SET_ID> \
  --items "$(cat ./eval_set_items.json)" \
  --format pretty

fornax-cli eval-set create-version \
  --id <EVAL_SET_ID> \
  --version "<EVAL_SET_VERSION>" \
  --format json
```

### 3) 创建评估器 + 发布版本

#### 3.1 Prompt Evaluator（推荐）

```bash
cat > prompt_evaluator.json <<'JSON'
{
  "name": "MyPromptEvaluator",
  "description": "Prompt evaluator created from CLI",
  "evaluator_type": "prompt",
  "current_version": {
    "version": "0.0.1",
    "description": "initial",
    "evaluator_content": {
      "prompt_evaluator": {
        "messages": [
          {
            "role": "system",
            "content": {
              "content_type": "text",
              "text": "你是一个严格的评测裁判。根据 input/expected_output/actual_output 给出 score(0-1) 和 reason。只输出 JSON：{\"score\":...,\"reason\":\"...\"}"
            }
          },
          {
            "role": "user",
            "content": {
              "content_type": "text",
              "text": "input: {{input}}\nexpected_output: {{expected_output}}\nactual_output: {{actual_output}}"
            }
          }
        ],
        "model_config": {
          "model_id": "7514544718487748609",
          "model_name": "doubao-seed-1.6-flash-250615",
          "temperature": 0.0,
          "max_tokens": 300
        }
      },
      "input_schemas": [
        {"key": "input", "support_content_types": ["text"], "json_schema": "{\"type\": \"string\"}"},
        {"key": "actual_output", "support_content_types": ["text"], "json_schema": "{\"type\": \"string\"}"},
        {"key": "expected_output", "support_content_types": ["text"], "json_schema": "{\"type\": \"string\"}"}
      ],
      "output_schemas": [
        {"key": "score"},
        {"key": "reason"}
      ]
    }
  }
}
JSON

fornax-cli evaluator create \
  --evaluator-file "./prompt_evaluator.json" \
  --output json
```

如果提示「当前空间已存在相同名称评估器」，改用 `--evaluator` 传入 JSON，并在 JSON 里把 `name` 改成唯一值再创建：

```bash
fornax-cli evaluator create \
  --evaluator "$(cat ./prompt_evaluator.json | python3 -c 'import json,sys,datetime; d=json.load(sys.stdin); d[\"name\"]=d.get(\"name\",\"Evaluator\")+\" [\"+datetime.datetime.now().strftime(\"%Y%m%d_%H%M%S\")+\"]\"; print(json.dumps(d,ensure_ascii=False))')" \
  --output json
```

发布版本（若 `1.0.0` 冲突则用 `1.0.1` 或更高）。**注意：`evaluator create` 返回的 `current_version` 是 draft 版本（版本号通常为 `evaluator_draft`），不能直接用于 `--evaluator <id>:<version>`。必须 `submit-version` 发布正式版本后才可引用**：

```bash
fornax-cli evaluator submit-version \
  --evaluator-id <PROMPT_EVAL_ID> \
  --version "<PROMPT_EVAL_VERSION>" \
  --description "stable" \
  --output json
```

#### 3.2 Code Evaluator（可选，兜底强）

```bash
cat > code_evaluator.json <<'JSON'
{
  "name": "MyCodeEvaluator",
  "description": "Code evaluator created from CLI",
  "evaluator_type": "code",
  "current_version": {
    "version": "0.0.1",
    "description": "initial",
    "evaluator_content": {
      "code_evaluator": {
        "language_type": "python",
        "code_content": "def exec_evaluation(turn):\n    expected = turn[\"evaluate_dataset_fields\"][\"expected_output\"][\"text\"].strip()\n    actual = turn[\"evaluate_target_output_fields\"][\"actual_output\"][\"text\"].strip()\n    score = 1.0 if actual == expected else 0.0\n    return EvalOutput(score=score, reason=\"match\" if score else f\"mismatch: actual='{actual}' expected='{expected}'\")\n"
      }
    }
  }
}
JSON

fornax-cli evaluator create \
  --evaluator-file "./code_evaluator.json" \
  --output json

fornax-cli evaluator submit-version \
  --evaluator-id <CODE_EVAL_ID> \
  --version "<CODE_EVAL_VERSION>" \
  --description "stable" \
  --output json
```

### 4) 提交评测实验（Experiment）

#### 4.1 coze\_loop\_prompt（最常用模板）

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
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:expected_postcode=expected_postcode \
  --evaluator-map-from-target  <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:actual_output=actual_output \
  --evaluator-map-from-evalset <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:expected_postcode=expected_postcode \
  --evaluator-map-from-target  <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:actual_output=actual_output \
  --item-retry-num 3 \
  --format json
```

#### 4.2 跳过评测对象（仅评估 eval-set 中已有数据）

适用：你的评测集里已经包含 evaluator 需要的所有输入字段（例如把“case\_url/agent\_output”等作为 eval-set 字段），不需要再执行 target。

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

注意：

- 不允许传 `--target-type/--target-id/--target-version/--target-map/--raw-target-field-mapping`
- 不允许使用 `--evaluator-map-from-target`（因为没有 target 输出）

#### 4.3 跳过评估器（仅执行 target，先产出 target 输出）

适用：你只想跑一次“推理执行/目标执行”，先把 target 输出产出来，后续再用 evaluator 做评分或聚合。

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

注意：

- 不允许传 `--evaluator` 与任何 evaluator mapping（`--evaluator-map-*` / `--raw-evaluator-field-mapping`）

输出里 `experiment.id` 作为 `<EXPERIMENT_ID>`。

### 5) 查看实验状态与配置（推荐：experiment detail）

当你想判断实验是否跑完、成功/失败计数、以及复用同一份 mapping 配置时，用 `experiment detail`：

```bash
fornax-cli experiment detail \
  --experiment-id <EXPERIMENT_ID> \
  --format json
```

关注字段：

- `experiment.status`：`pending / processing / success / failed / terminated`
- `experiment.expt_stats`：`pending_turn_count / processing_turn_count / success_turn_count / failed_turn_count / terminated_turn_count`
- `experiment.target_field_mapping / experiment.evaluator_field_mapping`：用于下一次 experiment submit 直接复用

### 6) 聚合指标与准出（experiment agg-results，用于效果/性能/成本）

> **前置条件**：`experiment agg-results` 在实验未完成时会报错 `resource not found, experiment aggr result not found`（不是返回空结果）。
> 必须先用 `experiment detail`（Step 5）确认实验状态到达终态（`success` / `failed` / `terminated`）后，再调用 `agg-results`。

`experiment agg-results` 用于拉取实验的聚合结果（open-api only），主要包含两类数据：

- `evaluator_results`：每个 evaluator 的聚合分数（`average / sum / max / min / distribution`）
- `eval_target_aggr_result`：目标侧的性能/成本聚合（`latency(ms)`、`input_tokens / output_tokens / total_tokens`，同样有 `average/sum/max/min/distribution`）

#### 6.1 拉取聚合结果

```bash
fornax-cli experiment agg-results \
  --experiment-id <EXPERIMENT_ID> \
  --format json
```

也可以把 JSON 保存到目录（文件名为 `<experiment-id>.json`）：

```bash
fornax-cli experiment agg-results ./output \
  --experiment-id <EXPERIMENT_ID>
```

#### 6.2 效果准出：平均分低于阈值则失败（例如 < 0.9）

下面示例会读取 `evaluator_results[*].aggregator_results` 里 `aggregator_type=="average"` 的值；
只要任意 evaluator 的平均分 `<0.9` 就以非 0 退出码退出（适合在 CI 里做“效果准出”）。

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

#### 6.3 性能/成本准出：基于 latency/tokens 做阈值判断

示例：限制平均耗时、平均总 tokens 上限（可按需改成 max/sum 等）。

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

### 7) 拉取实验结果明细（experiment results）

```bash
fornax-cli experiment results \
  --experiment-id <EXPERIMENT_ID> \
  --limit 100 \
  --format pretty
```

### 8) 增量追加评测集样本 + 发布新版本 + 再发起实验（常用迭代流程）

当你已经有一个评测集（`<EVAL_SET_ID>`），想“再多加几条正/难例数据，然后再跑一次实验”，推荐走增量流程：

#### 8.1 追加样本（add-items）

准备一个 items JSON（数组格式）：

```json
[
  {"input": "北京市海淀区清华园", "expected_postcode": "100084"},
  {"input": "山西省太原市迎泽区迎泽大街2号", "expected_postcode": "030001"}
]
```

然后追加：

```bash
cat > more_items.json <<'JSON'
[
  {"input": "北京市海淀区清华园", "expected_postcode": "100084"},
  {"input": "山西省太原市迎泽区迎泽大街2号", "expected_postcode": "030001"}
]
JSON

fornax-cli eval-set add-items \
  --id <EVAL_SET_ID> \
  --items "$(cat ./more_items.json)" \
  --format json
```

#### 8.2 发布评测集新版本（create-version）

注意：**新增 items 后一定要 create-version**，否则 experiment submit 使用旧版本看不到新增数据。

```bash
fornax-cli eval-set create-version \
  --id <EVAL_SET_ID> \
  --version "<NEW_EVAL_SET_VERSION>" \
  --format json
```

#### 8.3 用新版本再提交一个实验（experiment submit）

只需要把 `--eval-set-version` 换成新版本，其它 evaluator/target/mapping 通常不变：

```bash
fornax-cli experiment submit \
  --name "<EXPERIMENT_NAME_V2>" \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version "<NEW_EVAL_SET_VERSION>" \
  --evaluator "<PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>" \
  --evaluator "<CODE_EVAL_ID>:<CODE_EVAL_VERSION>" \
  --target-type coze_loop_prompt \
  --target-id <PROMPT_ID> \
  --target-version "<PROMPT_VERSION>" \
  --target-map query=input \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:input=input \
  --evaluator-map-from-evalset <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:expected_postcode=expected_postcode \
  --evaluator-map-from-target  <PROMPT_EVAL_ID>:<PROMPT_EVAL_VERSION>:actual_output=actual_output \
  --evaluator-map-from-evalset <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:expected_postcode=expected_postcode \
  --evaluator-map-from-target  <CODE_EVAL_ID>:<CODE_EVAL_VERSION>:actual_output=actual_output \
  --format json
```

随后用：

- `experiment detail` 查询状态，直到进入终态（success/failed/terminated）
- `experiment agg-results` 做聚合指标/准出判断
- `experiment results` 拉取明细结果定位具体失败样本

## 常见排障

- 字段名不确定：先查
  - `eval-set get --id <EVAL_SET_ID> --format json`
  - `evaluator get --id <EVAL_ID> --output json`
- Prompt evaluator 运行时报 `content type is not supported`：检查 evaluator JSON 的 `input_schemas` 每项都有 `support_content_types: ["text"]`（而不是 `content_type`）。
- `evaluator submit-version` 报 `API error: Unknown error`：通常是版本号冲突或非法。优先改用新版本号（例如 `0.0.2`、`0.0.3`），或直接不传 `--version` 让系统自动生成。
- experiment submit 报 `Invalid value for '--format': 'json~'`：一般是复制命令时混入了多余字符（例如末尾 `~`），删掉后重试。

