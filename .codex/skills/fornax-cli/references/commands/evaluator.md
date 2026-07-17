# fornax-cli evaluator

Evaluator 资源相关命令，包含：列表/详情/创建/更新/删除、提交版本、单次运行、执行预置评估器与批量查询 records。

## evaluator list

分页列出 evaluators，并支持可选过滤条件。

### 用法

```bash
fornax-cli evaluator list [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_list_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli evaluator list
fornax-cli evaluator list --name demo --type prompt --with-version
fornax-cli evaluator list --page-no 2 --page-size 50 -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--builtin`：仅返回内置 evaluator
- `--name <name>`：按 evaluator 名称模糊查询（匹配规则以服务端为准）
- `--type <type>`：按类型过滤（可重复传多次），可选值：`prompt`、`code`、`custom_rpc`
  - 示例：`--type prompt --type code`
- `--with-version`：在列表中附带 `current_version` 信息（便于直接看到当前版本）
- `--limit <N>`：最多返回 N 条；`0` 表示不限制
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_list_<YYYYMMDDHHMMSS>.json`

## evaluator get

按 id 获取 evaluator 详情。

### 用法

```bash
fornax-cli evaluator get [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_<id>.json`

### 示例

```bash
fornax-cli evaluator get --id <EVALUATOR_ID>
fornax-cli evaluator get --id <EVALUATOR_ID> -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVALUATOR_ID>`：evaluator id（必填）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_<id>.json`

## evaluator create

创建一个 evaluator。

### 用法

```bash
fornax-cli evaluator create [选项]
```

### 两种创建方式

#### 方式 1：直接给完整 JSON

- `--evaluator`：完整的 evaluator JSON 对象字符串
- `--evaluator-file`：完整的 evaluator JSON 文件路径

这两者会覆盖其他组合式选项（`--name/--type/...`）。

#### 方式 2：用常用选项组合（未提供完整 JSON 时生效）

- `--name` 必填
- `--type` 默认 `prompt`（可选值：`prompt`、`code`、`custom_rpc`）
- 当 `--type=prompt`：必须提供 `--prompt` 或 `--prompt-file`

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_<evaluator_id>.json`

### 示例

#### Prompt Evaluator（推荐，使用完整 JSON 文件创建）

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

fornax-cli evaluator create --evaluator-file ./prompt_evaluator.json --format json
```

> **注意**：`input_schemas` 每项必须有 `support_content_types: ["text"]`（不是 `content_type`），否则运行时会报 `content type is not supported`。

#### Code Evaluator（精确匹配场景）

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

fornax-cli evaluator create --evaluator-file ./code_evaluator.json --format json
```

#### 组合选项方式（简单场景）

```bash
fornax-cli evaluator create --name demo --type prompt --prompt "hello" -o ./out
```

#### 名称冲突处理

如果提示「当前空间已存在相同名称评估器」，在 JSON 里把 `name` 改成唯一值再创建：

```bash
fornax-cli evaluator create \
  --evaluator "$(cat ./prompt_evaluator.json | python3 -c 'import json,sys,datetime; d=json.load(sys.stdin); d[\"name\"]=d.get(\"name\",\"Evaluator\")+\" [\"+datetime.datetime.now().strftime(\"%Y%m%d_%H%M%S\")+\"]\"; print(json.dumps(d,ensure_ascii=False))')" \
  --format json
```

### 参数说明

#### 通用

- `-h, --help`：显示本子命令帮助信息并退出
- `--description <text>`：evaluator 描述（可选）

#### 方式 1：传完整 JSON（会覆盖其他组合式选项）

完整 JSON 必须是 JSON object。建议用单引号包裹字符串，或优先使用 `--evaluator-file` 避免转义问题。

- `--evaluator '<json_object>'`：直接传 evaluator JSON 对象字符串（覆盖其他选项）
- `--evaluator-file <path>`：从文件读取 evaluator JSON（覆盖其他选项）

#### 方式 2：组合选项（未提供完整 JSON 时生效）

- `--name <name>`：evaluator 名称（必填）
- `--type <type>`：类型，可选值：`prompt`、`code`、`custom_rpc`；默认 `prompt`
- `--version <ver>`：版本号字符串（可选）；默认 `1.0.0`
- `--version-description <text>`：版本描述（可选）
- `--input-schema '<json_array>'`：输入 schema（可选），必须是 JSON array 字符串

#### prompt 类型相关（`--type=prompt`）

当 `--type=prompt` 时，至少提供一个：

- `--prompt "<text>"`：prompt 文本（必填其一）
- `--prompt-file <path>`：prompt 文件路径（必填其一）

> **⚠️ 关键：prompt 中必须使用 `{{变量名}}` 模板语法引用输入字段**
>
> 评估器 prompt 中**必须使用 `{{变量名}}` 模板语法**（如 `{{input}}`、`{{output}}`）来引用输入字段。仅在自然语言中提及字段名（如"请根据 dish 和 review 评分"）**不会**被替换为实际数据，评估器运行时将无法获得评测集中的真实内容。
>
> - 使用 `--prompt` 快速创建时，评估器自动生成 `input` 和 `output` 两个 input key，因此 prompt 中应使用 `{{input}}` 和 `{{output}}`
> - 使用完整 JSON 创建时，`{{变量名}}` 需与 `input_schemas` 中的 `key` 一致
> - 评估器的 input key 与评测集列名**可以不同**，通过字段映射（`--evaluator-map-from-evalset`）在创建实验模板/提交实验时关联
>
> **正确示例**：`--prompt '标准答案是{{input}}，模型答案是{{output}}。判断是否正确。'`
>
> **错误示例**：`--prompt '根据 dish 和 review 评分'`（变量未用 `{{}}` 包裹，运行时不会注入数据）

模型与采样参数：

- `--model-id <id>`：模型 ID（仅在 type=prompt 时使用）；有默认值
- `--model-name <name>`：模型名称（仅在 type=prompt 时使用）；有默认值
- `--temperature <float>`：采样温度（默认 1）；示例：`0.1`、`1.0`
- `--max-tokens <int>`：最大输出 tokens（默认 1000）

#### 创建后行为

- `--resolve-version-id`：创建后是否再 fetch 一次 evaluator 以填充 `current_version_id`；默认 true

> **注意**：`evaluator create` 返回的 `current_version` 是 **draft 版本**（版本号通常为 `evaluator_draft`），不能直接用于 `experiment submit` 或 `experiment-template create` 的 `--evaluator <id>:<version>` 参数。必须先调用 `evaluator submit-version` 发布正式版本（如 `1.0.0`、`1.0.1`），才能在实验/模板中引用。

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_<evaluator_id>.json`

## evaluator update

更新 evaluator 的名称/描述（至少提供一个）。

### 用法

```bash
fornax-cli evaluator update [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_<id>.json`

### 示例

```bash
fornax-cli evaluator update --id <EVALUATOR_ID> --name new_name
fornax-cli evaluator update --id <EVALUATOR_ID> --description "new desc" -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVALUATOR_ID>`：evaluator id（必填）
- `--name <new_name>`：新名称（可选）
- `--description <text>`：新描述（可选）

注意：`--name` 与 `--description` 至少提供一个，否则无实际更新内容。

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_<id>.json`

## evaluator update-draft

更新 evaluator 的草稿版本内容（evaluator_content 和 evaluator_type）。

### 用法

```bash
fornax-cli evaluator update-draft [选项]
```

### 两种提供 evaluator_content 的方式

- `--content`：JSON object 字符串
- `--content-file`：JSON 文件路径（内容必须是 JSON object）

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_draft_<id>.json`

### 示例

```bash
fornax-cli evaluator update-draft --id <EVALUATOR_ID> --type prompt --content '{"prompt_evaluator":{...}}'
fornax-cli evaluator update-draft --id <EVALUATOR_ID> --type code --content-file ./content.json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVALUATOR_ID>`：evaluator id（必填）
- `--type <type>`：evaluator 类型（必填），可选值：`prompt`、`code`、`custom_rpc`
- `--content '<json_object>'`：evaluator content JSON 对象字符串（与 `--content-file` 二选一）
  - 建议用单引号包裹，避免 shell 转义
- `--content-file <path>`：从文件读取 evaluator content JSON（与 `--content` 二选一）

注意：`--content` 与 `--content-file` 至少提供一个，内容必须是 JSON object。

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_draft_<id>.json`

## evaluator delete

删除一个 evaluator。

### 用法

```bash
fornax-cli evaluator delete [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_delete_<id>.json`

### 示例

```bash
fornax-cli evaluator delete --id <EVALUATOR_ID>
fornax-cli evaluator delete --id <EVALUATOR_ID> -y -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVALUATOR_ID>`：evaluator id（必填）
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用，避免误删）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_delete_<id>.json`

## evaluator submit-version

提交一个新的 evaluator 版本。**这是将 evaluator 从 draft 状态发布为可用正式版本的必要步骤**——只有 submit-version 后的版本号才能用于 `experiment submit` 和 `experiment-template create` 的 `--evaluator <id>:<version>` 参数。

### 用法

```bash
fornax-cli evaluator submit-version [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_version_<version_id>.json`（如无法取到 version_id，则回退使用 evaluator_id 命名）

### 示例

```bash
fornax-cli evaluator submit-version --evaluator-id <EVALUATOR_ID> --version 1.0.1 --description "fix" -o ./out
fornax-cli evaluator submit-version --evaluator-id <EVALUATOR_ID> --description "auto version"
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--evaluator-id <EVALUATOR_ID>`：evaluator id（必填）
- `--version <ver>`：版本号字符串（可选），示例：`1.0.1`；不传则由系统自动生成
- `--description <text>`：版本描述（可选）

### 常见问题

- 报 `API error: Unknown error`：通常是**版本号冲突**（该版本号已存在）。改用新版本号（如 `0.0.2`、`0.0.3`），或不传 `--version` 让系统自动生成。
- 若 `1.0.0` 已被 `evaluator create` 的 `current_version` 占用，首次 `submit-version` 应使用 `1.0.1` 或更高版本。

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_version_<version_id>.json`（如无法取到 version_id，则回退使用 evaluator_id 命名）

## evaluator run

通过 version-id 执行一次 evaluator。

### 用法

```bash
fornax-cli evaluator run [选项]
```

### 输入

- `--input`：JSON object 字符串
- `--input-file`：JSON 文件路径（内容必须是 JSON object）
- `--config` / `--config-file`：可选的运行配置

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_run_<version_id>.json`

### 示例

```bash
fornax-cli evaluator run --version-id <VERSION_ID> --input '{"input":{"content_type":"text","text":"hi"}}'
fornax-cli evaluator run --version-id <VERSION_ID> --input-file ./input.json --config-file ./conf.json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--version-id <VERSION_ID>`：evaluator version id（必填）

#### 输入（建议二选一）

输入必须是 JSON object（不是 array）。

- `--input '<json_object>'`：直接传 input JSON 对象字符串
  - 建议用单引号包裹，避免 shell 转义
- `--input-file <path>`：从文件读取 input JSON（文件内容必须是 JSON object）

#### 配置（可选，二选一）

用于传递运行配置（JSON object），例如推理参数、运行时选项等（具体字段以服务端为准）。

- `--config '<json_object>'`：直接传 config JSON 对象字符串
- `--config-file <path>`：从文件读取 config JSON（文件内容必须是 JSON object）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_run_<version_id>.json`

## evaluator run-builtin

通过名称或 id 执行一次预置评估器（built-in evaluator）。

### 用法

```bash
fornax-cli evaluator run-builtin [选项]
```

### 标识（至少提供一个）

- `--name`：预置评估器名称（例如 TC260）
- `--builtin-id`：预置评估器 id
  - 若两者都传则需在服务端匹配

### 输入

- `--input`：JSON object 字符串
- `--input-file`：JSON 文件路径（内容必须是 JSON object）
- `--config` / `--config-file`：可选的运行配置

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_run_builtin_<name_or_id>.json`

### 示例

```bash
fornax-cli evaluator run-builtin --name TC260 --input '{"input_fields":{"model_input":{"content_type":"text","text":"hello"},"model_output":{"content_type":"text","text":"hi"}}}'
fornax-cli evaluator run-builtin --builtin-id 123 --input-file ./input.json -o ./out
fornax-cli evaluator run-builtin --name TC260 --input-file ./input.json --config-file ./conf.json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <NAME>`：预置评估器名称（例如 TC260）；与 `--builtin-id` 至少提供一个
- `--builtin-id <ID>`：预置评估器 id；与 `--name` 至少提供一个

#### 输入（建议二选一）

输入必须是 JSON object（不是 array）。

- `--input '<json_object>'`：直接传 input JSON 对象字符串
  - 建议用单引号包裹，避免 shell 转义
- `--input-file <path>`：从文件读取 input JSON（文件内容必须是 JSON object）

#### 配置（可选，二选一）

用于传递运行配置（JSON object），例如推理参数、运行时选项等（具体字段以服务端为准）。

- `--config '<json_object>'`：直接传 config JSON 对象字符串
- `--config-file <path>`：从文件读取 config JSON（文件内容必须是 JSON object）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_run_builtin_<name_or_id>.json`

## evaluator get-records

批量获取 evaluator records。

### 用法

```bash
fornax-cli evaluator get-records [选项]
```

### 输入

- `--record-ids`：用英文逗号分隔的 record id（必填），例如 `1,2,3`

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`evaluator_records_batch_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli evaluator get-records --record-ids 1,2,3
fornax-cli evaluator get-records --record-ids 1,2,3 --include-deleted -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--record-ids <id1,id2,...>`：record id 列表（必填），用英文逗号分隔，不要带空格
  - 示例：`--record-ids 1,2,3`
- `--include-deleted`：是否包含已删除的 records

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `evaluator_records_batch_<YYYYMMDDHHMMSS>.json`

## 端到端使用流程（创建 → 发布版本）

典型流程：先用完整 JSON 文件创建 evaluator（会包含初始版本），然后 submit-version 发布正式版本。

### 1) 创建 Evaluator

参考上面 `evaluator create` 中的 Prompt Evaluator 或 Code Evaluator 完整 JSON 示例。

```bash
fornax-cli evaluator create --evaluator-file ./prompt_evaluator.json --format json
# 记录输出里的 evaluator_id 作为 <EVALUATOR_ID>
# 记录输出里的 current_version_id 作为 <VERSION_ID>（用于 experiment submit）
```

### 2) 发布版本

```bash
fornax-cli evaluator submit-version \
  --evaluator-id <EVALUATOR_ID> \
  --version "<EVAL_VERSION>" \
  --description "stable" \
  --format json
```

### 常见排障

- Prompt evaluator 运行时报 `content type is not supported`：检查 evaluator JSON 的 `input_schemas` 每项都有 `support_content_types: ["text"]`（不是 `content_type`）。
- `evaluator submit-version` 报 `API error: Unknown error`：通常是版本号冲突或非法。优先改用新版本号（如 `0.0.2`、`0.0.3`），或不传 `--version` 让系统自动生成。
- 空间内名称重复报错：在 JSON 的 `name` 字段加上时间戳或后缀使其唯一。
