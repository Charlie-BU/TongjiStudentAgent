# fornax-cli model

模型管理命令，包含创建、更新、列表查询、详情获取。

## model create

创建模型及其账号。

### 用法

```bash
fornax-cli model create [选项]
```

### 参数说明

- `--data <JSON|@file>`：ModelAccount JSON 字符串或 `@file.json`（必选）
- `--dry-run`：仅打印请求体，不实际发送
- `--template`：输出完整 JSON 模板（含所有支持字段和必填标记），推荐先用 `--template` 生成模板再按需修改

### JSON 结构

```json
{
  "model": {
    "identification": "my-model",
    "displayName": "My Model",
    "provider": "OpenAI",
    "series": {"name": "Doubao", "family": "Doubao"},
    "description": "描述",
    "modelVendor": "字节跳动",
    "modelVersion": "v1.0",
    "actualName": "doubao-pro-32k",
    "modelTags": ["tag1", "tag2"],
    "ability": {
      "maxContextTokens": 32000,
      "maxInputTokens": 30000,
      "maxOutputTokens": 4096,
      "functionCallEnabled": true,
      "thinkingSwitchEnabled": false,
      "multiModalEnabled": false,
      "multiModalOutputEnabled": false,
      "jsonModeEnabled": true,
      "responseAPIEnabled": false
    },
    "visibility": {"mode": "Default"}
  },
  "accounts": [
    {
      "region": "CN",
      "usageScenario": "Default",
      "authorization": {
        "gptOpenAPI": { "ak": "your-api-key" }
      },
      "quota": {"qpm": 100, "tpm": 10000}
    }
  ]
}
```

**必填字段**：`model.identification`、`model.provider`、`model.family` 或 `model.series`（推荐 series）

**authorization 格式**：按 provider 选择对应的嵌套 key：
- GPTOpenAPI / OpenAI：`{"gptOpenAPI": {"ak": "your-api-key"}}`
- Maas：`{"maas": {"apiKey": "your-api-key"}}`

> 如果使用旧格式 `{"apiKey": "..."}` ，CLI 会自动根据 provider 转换为正确的嵌套格式。

`series` 是 `family` 的替代字段（推荐），结构：`{"name": "系列名", "icon": "图标URL", "family": "Family枚举值"}`

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`model_<id>.json`

### 示例

```bash
fornax-cli model create --template > model.json   # 生成模板
fornax-cli model create --data @model.json
fornax-cli model create --data @model.json --dry-run
fornax-cli model create --data @model.json -o ./out
```

## model update

更新模型信息或状态。

### 用法

```bash
fornax-cli model update [选项]
```

### 参数说明

- `--model-id <ID>`：Model ID（必选）
- `--space-id <ID>`：Space ID（可选，自动从 AK/SK 凭证检测）
- `--status <STATUS>`：目标状态（与 `--data` 互斥），见 [ModelStatus 枚举](#modelstatus)
- `--data <JSON|@file>`：ModelAccount JSON 字符串或 `@file.json`（与 `--status` 互斥）
- `--template`：输出完整 JSON 模板（同 create）

`--status` 和 `--data` 二选一：
- `--status`：仅更新模型状态（调用 update-status API）
- `--data`：更新模型完整信息（调用 upsert API）

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`model_<modelID>.json`

### 示例

```bash
# 仅更新状态
fornax-cli model update --model-id 67890 --status Available
fornax-cli model update --model-id 67890 --status 3

# 查看 JSON 模板
fornax-cli model update --template

# 更新模型信息（通过 upsert）
fornax-cli model update --model-id 67890 --data @model.json
fornax-cli model update --model-id 67890 --data @model.json -o ./out
```

## model list

列出空间内的模型。

### 用法

```bash
fornax-cli model list [选项]
```

### 参数说明

- `--space-id <ID>`：Space ID（可选，自动从 AK/SK 凭证检测）
- `--all`：获取所有模型（忽略分页参数，自动循环拉取）
- `--is-public`：列出公共模型，默认 `false`，仅展示私有模型
- `--status <STATUS>`：状态过滤（可重复，默认 `Available`），见 [ModelStatus 枚举](#modelstatus)
- `--page-num <N>`：页码（默认 1）
- `--page-size <N>`：每页条数（默认 20）

### Pretty 输出列

| 列 | 说明 |
|----|------|
| ID | 模型 ID |
| Name | 显示名称 |
| Identification | 模型标识 |
| Provider | 提供方（enum → string） |
| Status | 模型状态（enum → string） |
| Abilities | 模型能力：深度思考、多模态输入、多模态输出、函数调用、JSON模式、Response API |

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`model_list_<timestamp>.json`

### 示例

```bash
fornax-cli model list
fornax-cli model list --all
fornax-cli model list --status Available --status Deploying --page-size 50
fornax-cli model list --space-id 12345 --is-public -o ./out
```

## model get

获取模型详情，包含账号信息。

### 用法

```bash
fornax-cli model get [选项]
```

### 参数说明

- `--space-id <ID>`：Space ID（可选，自动从 AK/SK 凭证检测）
- `--model-id <ID>`：Model ID（必选）
- `--export`：导出可复用 JSON（移除服务端生成字段）

### Pretty 输出

输出包含两个表格：

**Model Detail**：ID、Name、Identification、Description、Provider、Family、Series、Status、Vendor、Version、Actual Name、Tags、Is Public、Created At（格式化时间）、Updated At（格式化时间）

**Accounts**：ID、Region、Usage Scenario、QPM（"0" 显示为 "不限流"）、TPM（"0" 显示为 "不限流"）

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`model_<modelID>.json`

### 示例

```bash
fornax-cli model get --model-id 67890
fornax-cli model get --space-id 12345 --model-id 67890 --format raw
fornax-cli model get --model-id 67890 -o ./out

# 导出可复用 JSON（移除 id 等服务端字段）
fornax-cli model get --model-id 67890 --export
fornax-cli model get --model-id 67890 --export -o ./out
```

## 通用说明

### 安全

所有读取类命令（`model list`、`model get`）的返回结果不包含 AK/SK、API Key 等敏感鉴权信息。写入类命令（`model create`、`model update`）的 `--data` 中可包含鉴权信息（`accounts[*].authorization`），请注意不要在日志或回复中泄露。

### Space ID 自动检测

所有需要 `--space-id` 的命令均支持自动检测：从 AK/SK 对应的 JWT 中提取 `workspace_id`。只有在需要访问非当前空间的模型时才需要手动指定。

### Enum 参考

所有 enum 参数同时接受字符串名称和整数值。

#### ModelStatus

| 字符串 | 整数 | 说明 |
|--------|------|------|
| `Available` | 1 | 健康可用 |
| `Deploying` | 2 | 部署中（系统管理，不可手动设置） |
| `Unavailable` | 3 | 已下线 |
| `Offlining` | 4 | 即将下线 |

#### Provider

| 字符串 | 整数 | 说明 |
|--------|------|------|
| `GPTOpenAPI` | 1 | GPT OpenAPI 平台 |
| `Maas` | 2 | 火山方舟 |
| `BotEngine` | 3 | bot_engine |
| `Merlin` | 4 | merlin 平台 |
| `MerlinSeed` | 5 | merlin-seed 平台 |
| `OpenAI` | 6 | OpenAI API 格式 |

#### Family

| 字符串 | 整数 | 字符串 | 整数 |
|--------|------|--------|------|
| `GPT` | 1 | `Doubao` | 12 |
| `Seed` | 2 | `Baichuan2` | 13 |
| `Gemini` | 3 | `DeepSeekV2` | 14 |
| `Claude` | 4 | `DeepSeekCoderV2` | 15 |
| `Ernie` | 5 | `DeepseekCoder` | 16 |
| `Baichuan` | 6 | `InternLM2_5` | 17 |
| `Qwen` | 7 | `Qwen2` | 18 |
| `GLM` | 8 | `Qwen2_5` | 19 |
| `SkyLark` | 9 | `Qwen2_5_Coder` | 20 |
| `Moonshot` | 10 | `MiniCPM` | 21 |
| `Minimax` | 11 | `MiniCPM3` | 22 |
| `ChatGLM3` | 23 | `Mistral` | 24 |
| `Gemma` | 25 | `Gemma2` | 26 |
| `InternVL2` | 27 | `InternVL2_5` | 28 |
| `DeepSeekV3` | 29 | `DeepSeekR1` | 30 |
| `Kimi` | 32 | `Seedream` | 33 |
| `InternVL3_5` | 34 | `Qwen3_5` | 35 |

#### Region

| 字符串 | 整数 |
|--------|------|
| `CN` | 1 |
| `SG` | 2 |
| `US` | 3 |

#### UsageScenario

| 字符串 | 整数 | 说明 |
|--------|------|------|
| `Default` | 1 | 默认场景 |
| `Evaluation` | 2 | 评测场景 |
| `PromptAsAService` | 3 | Prompt as a Service |
| `AIAnnotate` | 4 | AI 打标 |
| `AIScore` | 5 | 质量分 |
| `AITag` | 6 | 数据标签 |
| `DataSynthesis` | 7 | 数据合成 |

#### VisibleMode

| 字符串 | 整数 | 说明 |
|--------|------|------|
| `Default` | 1 | 仅模型所属空间可见 |
| `Specified` | 2 | 指定空间可见（需配合 spaceIDs） |
| `All` | 3 | 所有空间可见 |

#### DialogueInterfaceCategory

| 字符串 | 整数 | 说明 |
|--------|------|------|
| `ChatCompletionAPI` | 1 | Chat Completion API |
| `ResponseAPI` | 2 | Response API |
