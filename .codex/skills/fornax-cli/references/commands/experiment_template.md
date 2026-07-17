# fornax-cli experiment-template

实验模板（Experiment Template）相关命令，包含：创建模板、查询模板、列出模板、更新模板元信息、更新模板配置、基于模板提交实验。

实验模板预配置了评测设置（评测集、评估器、评测对象、字段映射等），方便重复提交实验时无需每次指定全部参数。

## experiment-template create

创建一个新的实验模板。

### 用法

```bash
fornax-cli experiment-template create [选项]
```

### 两种创建方式

1. **完整 JSON**：`--config` 或 `--config-file`（传入完整的模板配置 JSON object）
2. **组合参数**：通过 `--name`、`--eval-set-id`、`--eval-set-version-id`、`--evaluator`、`--target-type`、`--target-id` 等参数组合创建

### 必填（不使用 --config/--config-file 时）

- `--name`：模板名称
- `--eval-set-id`：评测集 id
- `--eval-set-version-id`：评测集版本
- `--evaluator`（除非设置 `--skip-evaluator`）
- `--target-type` / `--target-id`（除非设置 `--skip-target`）

### 字段映射

- 简单映射：`--target-map` / `--evaluator-map-from-evalset` / `--evaluator-map-from-target`
- 或直接传 raw JSON：`--raw-target-field-mapping` / `--raw-evaluator-field-mapping`

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_template_<template_id>.json`

### 示例

#### 基本用法

```bash
fornax-cli experiment-template create \
  --name demo_template \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version-id <VERSION_ID> \
  --evaluator <ID:VERSION> \
  --target-type coze_loop_prompt \
  --target-id <TARGET_ID> \
  -o ./out
```

#### 使用完整 JSON 配置

```bash
fornax-cli experiment-template create \
  --config-file ./template.json \
  -o ./out
```

#### 带字段映射

```bash
fornax-cli experiment-template create \
  --name demo_template \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version-id <VERSION_ID> \
  --evaluator <EVAL_ID>:<EVAL_VERSION> \
  --target-type coze_loop_prompt \
  --target-id <TARGET_ID> \
  --target-map query=input \
  --evaluator-map-from-evalset <EVAL_ID>:<EVAL_VERSION>:input=input \
  --evaluator-map-from-target <EVAL_ID>:<EVAL_VERSION>:actual_output=actual_output \
  --format json
```

### 参数说明

#### 组合参数

- `--name <name>`：模板名称（不使用 --config/--config-file 时必填）
- `--description <text>`：模板描述（可选）
- `--eval-set-id <id>`：评测集 id（不使用 --config/--config-file 时必填）
- `--eval-set-version-id <id>`：评测集版本 ID（不使用 --config/--config-file 时必填）。**注意：这是 `eval-set create-version` 返回的数字 ID（如 `7590091189405868546`），不是版本号字符串（如 `1.0.0`）**
- `--evaluator <id>:<version>`：评估器配置（可重复），格式：`<id>:<version>`
- `--skip-target`：创建不含评测对象的模板
- `--skip-evaluator`：创建不含评估器的模板
- `--target-type <type>`：target 类型，示例：`coze_loop_prompt`、`custom_rpc_server`、`coze_bot`、`coze_workflow`
- `--target-id <id>`：target 原始 ID（即 prompt/bot/workflow 本身的 ID，如 prompt_id）。CLI 会通过 `create_eval_target_param` 自动在评测系统中注册 target，无需手动查找评测系统内部 ID
- `--target-version <ver>`：target 版本（可选），即 prompt/bot/workflow 的已发布版本号（如 `1.0.0`）
- `--target-region <region>`：target 区域（默认 `cn`），可选值：`boe`、`cn`、`i18n`
- `--bot-info-type <type>`：bot info 类型（可选），可选值：`draft_bot`、`product_bot`
- `--bot-publish-version <ver>`：bot 发布版本（可选）
- `--env <env>`：target 运行环境（可选）
- `--custom-eval-target-json '<json>'`：自定义 eval target 配置（可选）
- `--concurrency <N>`：并发数（可选），`0` 表示服务端默认
- `--target-map`：target 字段映射（可重复）：`<target_field>=<eval_set_field>`
- `--evaluator-map-from-evalset`：评测集→评估器映射（可重复）：`<evaluator_id>:<version>:<field>=<evalset_field>`
- `--evaluator-map-from-target`：target→评估器映射（可重复）：`<evaluator_id>:<version>:<field>=<output_field>`
- `--raw-target-field-mapping`：target 字段映射 raw JSON
- `--raw-evaluator-field-mapping`：评估器字段映射 raw JSON array

#### 完整 JSON 配置

- `--config '<json>'`：完整模板配置 JSON object
- `--config-file <path>`：完整模板配置 JSON 文件路径

## experiment-template get

查询实验模板详情（支持单个或多个 ID）。

### 用法

```bash
fornax-cli experiment-template get --id <ID>[,<ID2>,...] [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_template_<id>.json`

### 示例

```bash
fornax-cli experiment-template get --id <TEMPLATE_ID>
fornax-cli experiment-template get --id <ID1>,<ID2> --format json
fornax-cli experiment-template get --id <TEMPLATE_ID> -o ./out
```

### 参数说明

- `--id <ids>`：模板 id（必填），多个用逗号分隔

## experiment-template list

分页列出实验模板。

### 用法

```bash
fornax-cli experiment-template list [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_template_list_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli experiment-template list
fornax-cli experiment-template list --name demo --page-no 1 --page-size 20
fornax-cli experiment-template list --format json -o ./out
```

### 参数说明

- `--name <name>`：模糊搜索模板名称
- `--limit <N>`：最多返回 N 条；`0` 表示不限制
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200

## experiment-template update-meta

更新实验模板的元信息（名称和/或描述）。

### 用法

```bash
fornax-cli experiment-template update-meta --id <TEMPLATE_ID> [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_template_<id>.json`

### 示例

```bash
fornax-cli experiment-template update-meta --id <TEMPLATE_ID> --name new_name
fornax-cli experiment-template update-meta --id <TEMPLATE_ID> --description "new description" -o ./out
```

### 参数说明

- `--id <id>`：模板 id（必填）
- `--name <name>`：新名称（可选，至少 --name 或 --description 之一必填）
- `--description <text>`：新描述（可选）

## experiment-template update

更新实验模板的完整配置（triple\_config、field\_mapping\_config、score\_weight\_config 等）。

### 用法

```bash
fornax-cli experiment-template update --id <TEMPLATE_ID> --config '<json>' [选项]
fornax-cli experiment-template update --id <TEMPLATE_ID> --config-file <path> [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_template_<id>.json`

### 示例

```bash
fornax-cli experiment-template update --id <TEMPLATE_ID> \
  --config '{"triple_config":{"eval_set_param":{"eval_set_id":"123","version":"1.0.0"}}}'

fornax-cli experiment-template update --id <TEMPLATE_ID> --config-file ./update.json -o ./out
```

### 参数说明

- `--id <id>`：模板 id（必填）
- `--config '<json>'`：更新内容 JSON object（与 --config-file 二选一）
- `--config-file <path>`：更新内容 JSON 文件路径（与 --config 二选一）

## experiment-template submit-expt

基于实验模板提交一个实验。

### 用法

```bash
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID> [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`experiment_<experiment_id>.json`

### 示例

```bash
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID>
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID> --name "my experiment" --description "run 1"
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID> --config '{"evaluator_params":[...]}' -o ./out
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID> --target-runtime-param '{"model_name":"glm5.0"}'
```

### 参数说明

- `--id <id>`：模板 id（必填）
- `--name <name>`：实验名称覆盖（可选）
- `--description <text>`：实验描述覆盖（可选）
- `--target-runtime-param '<json>'`：目标运行时参数 JSON object（可选）
- `--config '<json>'`：覆盖配置 JSON object（可选），支持字段：`evaluator_params`、`eval_target_param` 等
- `--config-file <path>`：覆盖配置 JSON 文件路径（可选）

## 典型工作流

1. 创建评测集 + 写入样本 + 发布版本
2. 创建评估器 + 发布版本（**prompt 中必须使用 `{{变量名}}` 引用输入字段**）
3. 创建实验模板（配置评测集、评估器、字段映射）
4. 基于模板反复提交实验（只需模板 id，可选覆盖名称/描述/部分参数）
5. 使用 `experiment detail` / `experiment results` / `experiment agg-results` 查看实验结果

### 端到端示例（skip-target 场景：评测集已包含所有数据）

以"数学加法正确率评测"为例：评测集包含题目、标准答案、模型答案，评估器判断模型答案是否正确。

#### Step 1：创建评测集 + 写入样本 + 发布版本

```bash
# 创建评测集（定义 schema）
fornax-cli eval-set create \
  --name "math_add_evalset" \
  --description "0-10 数学加法评测集" \
  --schema '{"field_schemas":[
    {"name":"question","key":"question","content_type":"text","text_schema":"{\"type\":\"string\"}"},
    {"name":"expected_answer","key":"expected_answer","content_type":"text","text_schema":"{\"type\":\"string\"}"},
    {"name":"model_answer","key":"model_answer","content_type":"text","text_schema":"{\"type\":\"string\"}"}
  ]}' \
  --format json
# 记录输出的 evaluation_set_id → <EVAL_SET_ID>

# 写入样本
fornax-cli eval-set add-items --id <EVAL_SET_ID> --items '[
  {"question":"1+2=?","expected_answer":"3","model_answer":"3"},
  {"question":"2+2=?","expected_answer":"4","model_answer":"5"},
  {"question":"3+5=?","expected_answer":"8","model_answer":"8"}
]' --format json

# 发布版本（必须在 add-items 之后）
fornax-cli eval-set create-version --id <EVAL_SET_ID> --version "1.0.0" --format json
# 记录输出的 version_id → <EVAL_SET_VERSION_ID>（这是数字 ID 如 7590091189405868546，不是版本号 "1.0.0"）
```

#### Step 2：创建评估器 + 发布版本

> **关键**：评估器 prompt 中 **必须使用 `{{变量名}}` 模板语法** 来引用输入字段，变量名对应评估器的 `input_schemas` 中的 key（如 `{{input}}`、`{{output}}`）。
> 仅在自然语言中提到字段名（如"标准答案 expected_answer"）是 **不会** 被替换为实际数据的。
>
> 评估器的 input key 与评测集列名 **可以不同**，通过字段映射（`--evaluator-map-from-evalset`）在创建模板/实验时关联。

推荐用 `--prompt` 快速创建（评估器自动生成 `input` 和 `output` 两个 input key）：

```bash
fornax-cli evaluator create \
  --name "math_correctness_evaluator" \
  --type prompt \
  --prompt '你是一个数学加法评分器。标准答案是{{input}}，模型答案是{{output}}。判断是否正确，正确返回 {"score":5}，错误返回 {"score":1}。' \
  --format json
# 记录输出的 evaluator_id → <EVAL_ID>

fornax-cli evaluator submit-version --evaluator-id <EVAL_ID> --version "1.0.1" --format json
# 记录输出的 version → <EVAL_VERSION>（如 1.0.1）
```

如需更复杂的评估器（自定义 input_schemas、多轮 message），参考 `submit_experiment.md` 中 Step 3 的完整 JSON 配置方式。

#### Step 3：创建实验模板（含字段映射）

字段映射格式：`<evaluator_id>:<version>:<evaluator_input_key>=<evalset_column_name>`

```bash
fornax-cli experiment-template create \
  --name "math_add_template" \
  --description "数学加法正确率评测模板" \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version-id <EVAL_SET_VERSION_ID> \
  --evaluator <EVAL_ID>:<EVAL_VERSION> \
  --evaluator-map-from-evalset <EVAL_ID>:<EVAL_VERSION>:input=expected_answer \
  --evaluator-map-from-evalset <EVAL_ID>:<EVAL_VERSION>:output=model_answer \
  --skip-target \
  --format json
# 记录输出的 experiment_template.meta.id → <TEMPLATE_ID>
```

> **映射说明**：
> - `input=expected_answer`：评估器的 `{{input}}` ← 评测集的 `expected_answer` 列
> - `output=model_answer`：评估器的 `{{output}}` ← 评测集的 `model_answer` 列

#### Step 4：基于模板提交实验

```bash
fornax-cli experiment-template submit-expt \
  --id <TEMPLATE_ID> \
  --name "数学加法评测 run_1" \
  --format json
# 记录输出的 experiment.id → <EXPERIMENT_ID>
```

#### Step 5：查看实验状态和结果

```bash
fornax-cli experiment detail --experiment-id <EXPERIMENT_ID> --format json
fornax-cli experiment agg-results --experiment-id <EXPERIMENT_ID> --format json
fornax-cli experiment results --experiment-id <EXPERIMENT_ID> --limit 100 --format pretty
```

### 简化工作流（已有评测集和评估器时）

当评测集、评估器已经创建好，只需从 Step 3 开始：

```bash
# 创建模板
fornax-cli experiment-template create \
  --name my_template \
  --eval-set-id <EVAL_SET_ID> \
  --eval-set-version-id <VERSION_ID> \
  --evaluator <EVAL_ID>:<EVAL_VERSION> \
  --target-type coze_loop_prompt \
  --target-id <TARGET_ID> \
  --target-map query=input \
  --evaluator-map-from-evalset <EVAL_ID>:<EVAL_VERSION>:input=input \
  --evaluator-map-from-target <EVAL_ID>:<EVAL_VERSION>:actual_output=actual_output \
  --format json

# 基于模板提交实验
fornax-cli experiment-template submit-expt --id <TEMPLATE_ID> --name "run_1" --format json

# 查看结果
fornax-cli experiment detail --experiment-id <EXPERIMENT_ID> --format json
fornax-cli experiment agg-results --experiment-id <EXPERIMENT_ID> --format json
```
