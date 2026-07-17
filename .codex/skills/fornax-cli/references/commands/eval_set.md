# fornax-cli eval-set

评测集（Eval Set）资源相关命令，包含：评测集本身、字段结构（schema/columns）、版本（versions）与条目（items）。

## 评测集（Eval Set）

### eval-set create

创建一个评测集。

#### 用法

```bash
fornax-cli eval-set create [选项]
```

#### 字段结构（Schema，选择其一）

- `--schema`：JSON object 字符串
- `--schema-file`：JSON 文件路径（内容必须是 JSON object）

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_<evaluation_set_id>.json`

#### 示例

先准备 schema 文件（推荐）：

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
```

然后创建评测集：

```bash
fornax-cli eval-set create --name demo --schema-file ./eval_set_schema.json
fornax-cli eval-set create --name demo --schema-file ./eval_set_schema.json -o ./out
```

也可以直接用 `--schema` 内联传 JSON（适合字段少的场景）：

```bash
fornax-cli eval-set create --name demo --schema '{"field_schemas":[{"name":"input","content_type":"text","is_required":true}]}'
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：评测集名称（必填）
- `--description <text>`：评测集描述（可选）

##### 字段结构（Schema，必选其一）

字段结构（Schema）用于描述评测集字段结构，必须是 JSON object（不是 array），核心字段为 `field_schemas` 数组，每项定义一个字段（列）。

常用字段属性：

| 属性 | 说明 | 示例值 |
|------|------|--------|
| `name` | 字段名（必填） | `"input"` |
| `description` | 字段描述（可选） | `"输入文本"` |
| `content_type` | 内容类型（必填） | `"text"` |
| `text_schema` | 文本字段的 JSON Schema（可选） | `"{\"type\": \"string\"}"` |
| `default_display_format` | 默认展示格式（可选） | `"plain_text"` |
| `is_required` | 是否必填（可选，默认 false） | `true` |

- `--schema '<json_object>'`：直接传 schema 的 JSON 对象字符串
  - 建议用单引号包裹，避免 shell 对 `"`、`{}` 等字符转义
  - 示例：`'{"field_schemas":[{"name":"input","content_type":"text","is_required":true}]}'`
- `--schema-file <path>`：从文件读取 schema（文件内容必须是 JSON object）
  - 适合 schema 较大或包含复杂转义时使用（推荐）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_<evaluation_set_id>.json`

### eval-set get

按 id 获取评测集详情。

#### 用法

```bash
fornax-cli eval-set get [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_<id>.json`

#### 示例

```bash
fornax-cli eval-set get --id <EVAL_SET_ID>
fornax-cli eval-set get --id <EVAL_SET_ID> -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_<id>.json`

### eval-set list

分页列出评测集。

**推荐使用 `--page-token` 进行翻页**，而不是 `--page-no`。`--page-no` 需要逐页跳过，效率极低；`--page-token` 直接定位到下一页，速度更快。

#### 用法

```bash
fornax-cli eval-set list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_list_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli eval-set list
fornax-cli eval-set list --name demo --page-size 20
fornax-cli eval-set list --page-token <next_page_token> --page-size 20
fornax-cli eval-set list -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：按名称过滤（模糊/前缀匹配以服务端实现为准）；不传则不过滤
- `--limit <N>`：最多返回 N 条；`0` 表示不限制（可能返回较多数据）
- `--page-no <N>`：页码，从 1 开始；默认 1。**不推荐使用**，效率极低，建议使用 `--page-token`
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200
- `--page-token <token>`：分页 token（**推荐**）；把上一次 list 返回的 `next_page_token` 原样传回，用于继续拉取下一页

#### 输出字段

响应中包含以下分页信息：
- `has_more`：是否还有更多数据
- `next_page_token`：下一页的 token，传入 `--page-token` 即可获取下一页

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_list_<YYYYMMDDHHMMSS>.json`

### eval-set update

更新评测集的名称/描述（至少提供一个）。

#### 用法

```bash
fornax-cli eval-set update [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_<id>.json`

#### 示例

```bash
fornax-cli eval-set update --id <EVAL_SET_ID> --name new_name
fornax-cli eval-set update --id <EVAL_SET_ID> --description "new desc" -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--name <new_name>`：新名称（可选）
- `--description <text>`：新描述（可选）

注意：`--name` 与 `--description` 至少提供一个，否则无实际更新内容。

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_<id>.json`

### eval-set delete

删除一个评测集。

#### 用法

```bash
fornax-cli eval-set delete [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_delete_<id>.json`

#### 示例

```bash
fornax-cli eval-set delete --id <EVAL_SET_ID>
fornax-cli eval-set delete --id <EVAL_SET_ID> -y -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用，避免误删）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_delete_<id>.json`

## 字段（Schema/Columns）

### eval-set update-schema

更新评测集 schema。

#### 用法

```bash
fornax-cli eval-set update-schema [选项]
```

#### 字段（Columns，选择其一）

- `--columns`：JSON array 字符串
- `--columns-file`：JSON 文件路径（内容必须是 JSON array）

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_schema_<id>.json`

#### 示例

```bash
fornax-cli eval-set update-schema --id <EVAL_SET_ID> --columns '[{"name":"input","content_type":"text"}]' -o ./out
fornax-cli eval-set update-schema --id <EVAL_SET_ID> --columns-file ./columns.json
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）

##### 字段（Columns，必选其一）

字段（Columns）用于定义/更新评测集字段（列）结构，必须是 JSON array（不是 object）。

- `--columns '<json_array>'`：直接传 columns 的 JSON 数组字符串
  - 建议用单引号包裹，避免 shell 转义
  - 示例：`'[{"name":"input","content_type":"text"}]'`
- `--columns-file <path>`：从文件读取 columns（文件内容必须是 JSON array）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_schema_<id>.json`

## 版本（Versions）

### eval-set create-version

创建一个评测集版本。

#### 用法

```bash
fornax-cli eval-set create-version [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_version_<version_id>.json`

#### 示例

```bash
fornax-cli eval-set create-version --id <EVAL_SET_ID> --version 1.0.0 --description "first" -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--version <ver>`：版本号字符串（必填），示例：`1.0.0`
- `--description <text>`：版本描述（可选）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_version_<version_id>.json`

### eval-set list-versions

分页列出评测集版本。

#### 用法

```bash
fornax-cli eval-set list-versions [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_versions_<id>.json`

#### 示例

```bash
fornax-cli eval-set list-versions --id <EVAL_SET_ID>
fornax-cli eval-set list-versions --id <EVAL_SET_ID> --page-size 50 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--limit <N>`：最多返回 N 条；`0` 表示不限制
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200
- `--page-token <token>`：分页 token（可选）；通常把上一次 list 返回的 page_token 原样传回（具体行为以服务端返回为准）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_versions_<id>.json`

## 条目（Items）

### eval-set add-items

向评测集新增 items。

#### 用法

```bash
fornax-cli eval-set add-items [选项]
```

#### 输入

- `--items`：items 的 JSON array 字符串（必填）

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_items_add_<id>.json`

#### 示例

简写格式（CLI 自动转换为 `turns/field_datas` 结构，推荐）：

```bash
fornax-cli eval-set add-items --id <EVAL_SET_ID> \
  --items '[{"input":"Hello","expected_output":"Hi"},{"input":"What is 2+2?","expected_output":"4"}]'
```

也可以用 `field_values` 格式：

```bash
fornax-cli eval-set add-items --id <EVAL_SET_ID> \
  --items '[{"field_values":{"input":"hi","output":"ok"}}]'
```

或完整 `turns/field_datas` 格式：

```bash
fornax-cli eval-set add-items --id <EVAL_SET_ID> \
  --items '[{"turns":[{"field_datas":[{"name":"input","content":{"content_type":"text","text":"hi"}}]}]}]' \
  -o ./out
```

从文件读取 items（推荐条目较多时使用）：

```bash
cat > eval_set_items.json <<'JSON'
[
  {"input": "Hello", "expected_output": "Hi"},
  {"input": "What is 2+2?", "expected_output": "4"}
]
JSON

fornax-cli eval-set add-items --id <EVAL_SET_ID> \
  --items "$(cat ./eval_set_items.json)" \
  --format json
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--items '<json_array>'`：要新增的 items，必须是 JSON array 字符串（必填）
  - 建议使用单引号包裹 JSON，避免 shell 转义
  - 支持三种 JSON 格式（CLI 自动识别并转换）：
    - **简写格式**（推荐）：`[{"field_name":"value", ...}]`，直接用字段名作 key
    - **field\_values 格式**：`[{"field_values":{"field_name":"value"}}]`
    - **完整 turns 格式**：`[{"turns":[{"field_datas":[{"name":"...","content":{"content_type":"text","text":"..."}}]}]}]`

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_items_add_<id>.json`

### eval-set list-items

分页列出评测集 items。

**推荐使用 `--page-token` 进行翻页**，而不是 `--page-no`。`--page-no` 需要逐页跳过，效率极低；`--page-token` 直接定位到下一页，速度更快。

注意：`--version` 与 `--version-id` 互斥，不能同时使用。推荐使用 `--version-id`，性能更好。

#### 用法

```bash
fornax-cli eval-set list-items [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_items_<id>[_<version>].json`

#### 示例

```bash
fornax-cli eval-set list-items --id <EVAL_SET_ID>
fornax-cli eval-set list-items --id <EVAL_SET_ID> --version 1.0.0 --page-size 50 -o ./out
fornax-cli eval-set list-items --id <EVAL_SET_ID> --version-id 12345 --page-size 50 -o ./out
fornax-cli eval-set list-items --id <EVAL_SET_ID> --page-token <next_page_token> --page-size 20
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--version <ver>`：评测集版本号字符串（可选），示例：`1.0.0`；不传则由服务端决定返回哪个版本的数据
- `--version-id <id>`：评测集版本 id（可选）；使用 `list-versions` 命令获取版本 id；与 `--version` 互斥
- `--limit <N>`：最多返回 N 条；`0` 表示不限制
- `--page-no <N>`：页码，从 1 开始；默认 1。**不推荐使用**，效率极低，建议使用 `--page-token`
- `--page-size <N>`：每页条数；默认 20；建议范围 1~200
- `--page-token <token>`：分页 token（**推荐**）；把上一次 list-items 返回的 `next_page_token` 原样传回，用于继续拉取下一页

#### 输出字段

响应中包含以下分页信息：
- `has_more`：是否还有更多数据
- `next_page_token`：下一页的 token，传入 `--page-token` 即可获取下一页

注意：`--version` 与 `--version-id` 互斥，不能同时使用。推荐使用 `--version-id`，性能更好。

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_items_<id>[_<version>].json`

### eval-set update-items

更新评测集 items。

#### 用法

```bash
fornax-cli eval-set update-items [选项]
```

#### 输入

- `--items`：items 的 JSON array 字符串（必填）

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_items_update_<id>.json`

#### 示例

```bash
fornax-cli eval-set update-items --id <EVAL_SET_ID> --items '[{"id":1,"turns":[...]}]' -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--items '<json_array>'`：要更新的 items，必须是 JSON array 字符串（必填）
  - 建议使用单引号包裹 JSON，避免 shell 转义
  - item 内通常需要包含可被服务端识别的 item id/结构（示例里包含 `id`）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_items_update_<id>.json`

### eval-set delete-items

删除评测集 items。

#### 用法

```bash
fornax-cli eval-set delete-items [选项]
```

#### 输入

- `--item-ids`：用英文逗号分隔的 item id（必填），例如 `1,2,3`

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`eval_set_items_delete_<id>.json`

#### 示例

```bash
fornax-cli eval-set delete-items --id <EVAL_SET_ID> --item-ids 1,2,3 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <EVAL_SET_ID>`：评测集 id（必填）
- `--item-ids <id1,id2,...>`：要删除的 item id 列表（必填），用英文逗号分隔，不要带空格
  - 示例：`--item-ids 1,2,3`

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `eval_set_items_delete_<id>.json`

## 端到端使用流程（创建 → 写入样本 → 发布版本）

典型流程：先创建评测集定义 schema，然后写入样本数据，最后发布版本用于实验。

### 1) 创建评测集

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

fornax-cli eval-set create \
  --name "<eval_set_name>" \
  --schema-file "./eval_set_schema.json" \
  --format json
# 记录输出里的 evaluation_set_id 作为 <EVAL_SET_ID>
```

### 2) 写入样本

```bash
cat > eval_set_items.json <<'JSON'
[
  {"input": "Hello", "expected_output": "Hi"},
  {"input": "What is 2+2?", "expected_output": "4"}
]
JSON

fornax-cli eval-set add-items \
  --id <EVAL_SET_ID> \
  --items "$(cat ./eval_set_items.json)" \
  --format pretty
```

### 3) 发布版本

```bash
fornax-cli eval-set create-version \
  --id <EVAL_SET_ID> \
  --version "<EVAL_SET_VERSION>" \
  --format json
```

> **注意**：新增 items 后一定要 `create-version`，否则 `experiment submit` 使用旧版本看不到新增数据。

### 4) 增量追加 + 发布新版本（迭代流程）

当已有评测集需要追加更多样本并重新实验时：

```bash
fornax-cli eval-set add-items \
  --id <EVAL_SET_ID> \
  --items '[{"input":"新的测试输入","expected_output":"期望结果"}]' \
  --format json

fornax-cli eval-set create-version \
  --id <EVAL_SET_ID> \
  --version "<NEW_EVAL_SET_VERSION>" \
  --format json
```

然后在 `experiment submit` 时使用新版本号即可。
