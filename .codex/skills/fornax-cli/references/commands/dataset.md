# fornax-cli dataset

数据集（Dataset）资源相关命令，包含：数据集本身、版本（versions）、数据项（items）与导入导出（import/export）。

## 数据集（Dataset）

### dataset list

搜索/列出数据集，支持游标分页。

#### 用法

```bash
fornax-cli dataset list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_list_<timestamp>.json`

#### 示例

```bash
fornax-cli dataset list
fornax-cli dataset list --name my_dataset
fornax-cli dataset list --cursor <CURSOR> --limit 20 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：按名称过滤（模糊搜索）；不传则不过滤
- `--cursor <cursor>`：分页游标，来自上一次 list 返回的 nextCursor
- `--limit <N>`：最多返回 N 条；`0` 表示不限制

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_list_<timestamp>.json`

### dataset create

创建一个普通数据集。

#### 用法

```bash
fornax-cli dataset create [选项]
```

#### 字段定义（Fields，可选，选择其一）

- `--fields`：JSON array 字符串
- `--fields-file`：JSON 文件路径（内容必须是 JSON array）

字段定义示例：
```json
[{"name":"input","contentType":1,"defaultFormat":1,"schemaKey":1},{"name":"output","contentType":1,"defaultFormat":1,"schemaKey":1}]
```

枚举值说明：
- contentType: 1=Text, 100=Multipart
- defaultFormat: 1=PlainText, 2=Markdown, 3=JSON, 4=YAML, 5=Code
- schemaKey: 1=String, 2=Integer, 3=Float, 4=Bool, 5=Message, 6=SingleChoice, 7=Trajectory

不传 `--fields` 时，系统自动创建默认字段 input / output。

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_<id>.json`

#### 示例

```bash
fornax-cli dataset create --name my_dataset
fornax-cli dataset create --name my_dataset --fields '[{"name":"input","contentType":1,"schemaKey":1},{"name":"output","contentType":1,"schemaKey":1}]'
fornax-cli dataset create --name my_dataset --fields-file ./fields.json
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：数据集名称（必填）
- `--description <text>`：数据集描述（可选）

##### 字段定义（Fields，可选，选择其一）

字段定义（Fields）用于描述数据集的列结构，必须是 JSON array（不是 object）。

- `--fields '<json_array>'`：直接传字段定义的 JSON 数组字符串
  - 建议用单引号包裹，避免 shell 对 `"`、`{}` 等字符转义
  - 示例：`'[{"name":"input","contentType":1,"schemaKey":1}]'`
- `--fields-file <path>`：从文件读取字段定义（文件内容必须是 JSON array）
  - 适合字段定义较大或包含复杂转义时使用

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_<id>.json`

## 版本（Versions）

### dataset list-versions

列出数据集的版本。

#### 用法

```bash
fornax-cli dataset list-versions [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_versions_<id>.json`

#### 示例

```bash
fornax-cli dataset list-versions --id <DATASET_ID>
fornax-cli dataset list-versions --id <DATASET_ID> --limit 10 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--cursor <cursor>`：分页游标（可选）
- `--limit <N>`：最多返回 N 条；`0` 表示不限制

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_versions_<id>.json`

### dataset create-version

创建数据集版本快照。

#### 用法

```bash
fornax-cli dataset create-version [选项]
```

版本号必须是 SemVer2 三段格式（如 1.0.0），且必须大于前一个版本号。

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_version_<versionID>.json`

#### 示例

```bash
fornax-cli dataset create-version --id <DATASET_ID> --version 1.0.0
fornax-cli dataset create-version --id <DATASET_ID> --version 1.0.1 --description "second release" -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--version <ver>`：版本号字符串（必填），示例：`1.0.0`
- `--description <text>`：版本描述（可选）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_version_<versionID>.json`

## 数据项（Items）

### dataset list-items

分页列出数据集数据项。

#### 用法

```bash
fornax-cli dataset list-items [选项]
```

不传 `--version` 或 `--version-id` 时，列出草稿（uncommitted）版本的数据项。
传 `--version`（semver）时，CLI 自动解析为 version ID。

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_items_<id>[_<version>].json`

#### 示例

```bash
fornax-cli dataset list-items --id <DATASET_ID>
fornax-cli dataset list-items --id <DATASET_ID> --version 1.0.0
fornax-cli dataset list-items --id <DATASET_ID> --version-id <VERSION_ID> --limit 100 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--version <ver>`：版本字符串（可选），示例：`1.0.0`；CLI 自动解析为 version ID
- `--version-id <VERSION_ID>`：内部版本 ID（可选，与 `--version` 二选一）
- `--cursor <cursor>`：分页游标（可选）
- `--limit <N>`：最多返回 N 条；`0` 表示不限制

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_items_<id>[_<version>].json`

### dataset get-item

按 item ID 获取单条数据项。

#### 用法

```bash
fornax-cli dataset get-item [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_item_<itemID>.json`

#### 示例

```bash
fornax-cli dataset get-item --id <DATASET_ID> --item-id <ITEM_ID>
fornax-cli dataset get-item --id <DATASET_ID> --item-id <ITEM_ID> --with-lineage -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--item-id <ITEM_ID>`：数据项 id（必填）
- `--with-lineage`：包含数据溯源（来源追踪）信息（可选，默认 false）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_item_<itemID>.json`

### dataset add-items

向数据集草稿版本批量新增数据项。

#### 用法

```bash
fornax-cli dataset add-items [选项]
```

#### 输入（选择其一）

- `--items`：JSON array 字符串
- `--items-file`：JSON 文件路径

支持的数据项格式：
- 标准格式：`[{"data":[{"name":"input","contentType":1,"content":"hello"}]}]`
- 简写格式：`[{"input":"hello","output":"world"}]`（自动转换为 data 数组）
- 带 key：`[{"itemKey":"key1","input":"hello"}]`

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_items_add_<id>.json`

#### 示例

```bash
fornax-cli dataset add-items --id <DATASET_ID> --items '[{"input":"hello","output":"world"}]'
fornax-cli dataset add-items --id <DATASET_ID> --items-file ./items.json --allow-partial
```

#### 使用例子

**带 itemKey**（幂等写入，相同 key 不会重复插入）：

```bash
fornax-cli dataset add-items \
  --id 7590082829145784578 \
  --items '[{"itemKey":"qa_001","input":"你好","output":"你好！"},{"itemKey":"qa_002","input":"再见","output":"再见！"}]'
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--items '<json_array>'`：要新增的数据项，必须是 JSON array 字符串
  - 建议使用单引号包裹 JSON，避免 shell 转义
- `--items-file <path>`：从文件读取数据项（文件内容必须是 JSON array）
- `--skip-invalid`：跳过无效数据项（默认：false）
- `--allow-partial`：容量限制时允许部分写入（默认：false）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_items_add_<id>.json`

### dataset update-item

更新单条数据项的内容（草稿版本）。

#### 用法

```bash
fornax-cli dataset update-item [选项]
```

#### 输入（选择其一）

- `--data`：JSON array 字符串，字段更新内容
- `--data-file`：JSON 文件路径

示例数据：`[{"name":"input","content":"new value"}]`

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_item_update_<itemID>.json`

#### 示例

```bash
fornax-cli dataset update-item --id <DATASET_ID> --item-id <ITEM_ID> --data '[{"name":"input","content":"new value"}]'
fornax-cli dataset update-item --id <DATASET_ID> --item-id <ITEM_ID> --data-file ./update.json
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--item-id <ITEM_ID>`：数据项 id（必填）
- `--data '<json_array>'`：字段更新 JSON 数组字符串
  - 建议使用单引号包裹 JSON，避免 shell 转义
- `--data-file <path>`：从文件读取字段更新（文件内容必须是 JSON array）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_item_update_<itemID>.json`

### dataset delete-items

批量删除数据集草稿版本中的数据项。

#### 用法

```bash
fornax-cli dataset delete-items [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_items_delete_<id>.json`

#### 示例

```bash
fornax-cli dataset delete-items --id <DATASET_ID> --item-ids id1,id2,id3
fornax-cli dataset delete-items --id <DATASET_ID> --item-ids id1,id2 -y
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--item-ids <id1,id2,...>`：要删除的数据项 id 列表（必填），用英文逗号分隔，不要带空格
  - 示例：`--item-ids id1,id2,id3`
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用，避免误删）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_items_delete_<id>.json`

### dataset clear-items

清空数据集草稿版本的全部数据项。

#### 用法

```bash
fornax-cli dataset clear-items [选项]
```

WARNING: 此操作会永久删除草稿版本的所有数据。

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_items_clear_<id>.json`

#### 示例

```bash
fornax-cli dataset clear-items --id <DATASET_ID>
fornax-cli dataset clear-items --id <DATASET_ID> -y
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用，操作不可逆）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_items_clear_<id>.json`

## 导入导出（Import/Export）

### dataset export

导出数据集到 HDFS 或下载到本地（异步任务）。

#### 用法

```bash
fornax-cli dataset export [选项]
```

不传 `--version` 或 `--version-id` 时，导出草稿版本。

导出模式（二选一）：
- `--hdfs-path`：导出到 HDFS 目录（provider=3），路径必须以 `/home` 开头，且可被 stone.fornax.ml_flow 访问
- `--local`：下载导出文件到本地（provider=4），任务完成后返回下载 URL

文件格式：jsonl（默认）、parquet、csv

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_export_<id>.json`

#### 示例

```bash
fornax-cli dataset export --id <DATASET_ID> --hdfs-path /home/myproject/exports --file-format jsonl
fornax-cli dataset export --id <DATASET_ID> --version 1.0.0 --hdfs-path /home/myproject/exports --wait
fornax-cli dataset export --id <DATASET_ID> --local --file-format jsonl --wait
```

#### 使用例子

导出为 parquet 格式：

```bash
fornax-cli dataset export \
  --id 7590082829145784578 \
  --version 1.0.0 \
  --hdfs-path /home/myproject/exports \
  --file-format parquet \
  --wait
```

下载到本地，等待完成后返回下载 URL：

```bash
fornax-cli dataset export \
  --id 7590082829145784578 \
  --version 1.0.0 \
  --local \
  --file-format jsonl \
  --wait
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--version <ver>`：版本字符串（可选），示例：`1.0.0`；不传则导出草稿版本
- `--version-id <VERSION_ID>`：内部版本 ID（可选，与 `--version` 二选一）
- `--hdfs-path <path>`：HDFS 目标目录路径，以 `/home` 开头（与 `--local` 互斥）
- `--local`：下载导出文件到本地（与 `--hdfs-path` 互斥）
- `--file-format <fmt>`：文件格式：`jsonl`（默认）、`parquet`、`csv`
- `--wait`：等待任务完成（每 3s 轮询，最长 10 分钟）

注意：`--hdfs-path` 与 `--local` 必须二选一，不能同时指定或同时不指定。

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_export_<id>.json`

### dataset import

从 HDFS 文件导入数据到数据集草稿版本（异步任务）。

#### 用法

```bash
fornax-cli dataset import [选项]
```

字段映射（field mapping）用于将源文件列名映射到数据集字段名，通过 `--field-mapping` 指定，可多次使用。

文件格式：jsonl（默认）、parquet、csv

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_import_<id>.json`

#### 示例

```bash
fornax-cli dataset import --id <DATASET_ID> --hdfs-path /home/myproject/data.jsonl \
  --field-mapping input=input --field-mapping output=output
fornax-cli dataset import --id <DATASET_ID> --hdfs-path /home/myproject/data.parquet \
  --file-format parquet --field-mapping text=input --overwrite --wait
```

#### 使用例子

源字段名与数据集字段名不同时，通过映射转换：

```bash
fornax-cli dataset import \
  --id 7590082829145784578 \
  --hdfs-path /home/myproject/data.parquet \
  --file-format parquet \
  --field-mapping question=input \
  --field-mapping answer=output \
  --wait
```

覆盖已有草稿数据（而非追加）：

```bash
fornax-cli dataset import \
  --id 7590082829145784578 \
  --hdfs-path /home/myproject/data.jsonl \
  --field-mapping input=input \
  --field-mapping output=output \
  --overwrite \
  --wait
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--hdfs-path <path>`：HDFS 源文件路径，以 `/home` 开头（必填）
- `--file-format <fmt>`：文件格式：`jsonl`（默认）、`parquet`、`csv`
- `--field-mapping <source=target>`：字段映射，格式为 `source=target`（必填，至少一个）；可多次使用以映射多个字段
  - 示例：`--field-mapping input=input --field-mapping output=output`
- `--overwrite`：覆盖已有草稿数据（默认：追加）
- `--wait`：等待任务完成（每 3s 轮询，最长 10 分钟）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_import_<id>.json`

## 任务管理（Jobs）

### dataset get-job

查询导入/导出任务的状态和进度。

#### 用法

```bash
fornax-cli dataset get-job [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_job_<jobID>.json`

#### 示例

```bash
fornax-cli dataset get-job --job-id <JOB_ID>
fornax-cli dataset get-job --job-id <JOB_ID> -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--job-id <JOB_ID>`：任务 ID（必填）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_job_<jobID>.json`

### dataset cancel-job

取消正在运行的导入/导出任务。

#### 用法

```bash
fornax-cli dataset cancel-job [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_job_cancel_<jobID>.json`

#### 示例

```bash
fornax-cli dataset cancel-job --job-id <JOB_ID>
fornax-cli dataset cancel-job --job-id <JOB_ID> -y
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--job-id <JOB_ID>`：任务 ID（必填）
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_job_cancel_<jobID>.json`

## Schema 管理

### dataset append-schema-fields

向数据集追加新的 schema 字段定义（append-only，不修改或删除已有字段）。新字段的 `key` 必须为空。

#### 用法

```bash
fornax-cli dataset append-schema-fields [选项]
```

#### 字段定义（Fields，必填，选择其一）

- `--fields`：JSON array 字符串
- `--fields-file`：JSON 文件路径（内容必须是 JSON array）

字段定义示例：
```json
[{"name":"score","description":"评分列","contentType":1,"defaultFormat":1,"schemaKey":1}]
```

枚举值说明：
- contentType: 1=Text, 100=MultiPart
- defaultFormat: 1=PlainText, 2=Markdown, 3=JSON, 4=YAML, 5=Code
- schemaKey: 1=String, 2=Integer, 3=Float, 4=Bool, 5=Message, 6=SingleChoice, 7=Trajectory

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`dataset_append_schema_<id>.json`

#### 示例

```bash
fornax-cli dataset append-schema-fields --id <DATASET_ID> --fields '[{"name":"score","contentType":1}]'
fornax-cli dataset append-schema-fields --id <DATASET_ID> --fields-file ./new_fields.json
```

#### 使用例子

追加单个文本列：

```bash
fornax-cli dataset append-schema-fields \
  --id 7590082829145784578 \
  --fields '[{"name":"remark","description":"备注列","contentType":1,"schemaKey":1}]'
```

追加多个列：

```bash
fornax-cli dataset append-schema-fields \
  --id 7590082829145784578 \
  --fields '[{"name":"col_a","contentType":1,"schemaKey":1},{"name":"col_b","contentType":1,"schemaKey":2}]'
```

追加轨迹类型字段（schemaKey=7）：

```bash
fornax-cli dataset append-schema-fields \
  --id 7590082829145784578 \
  --fields '[{"name":"trajectory_col","contentType":1,"schemaKey":7}]'
```

追加多模态类型字段（contentType=100）：

```bash
fornax-cli dataset append-schema-fields \
  --id 7590082829145784578 \
  --fields '[{"name":"multimodal_col","contentType":100}]'
```

从文件读取字段定义：

```bash
fornax-cli dataset append-schema-fields \
  --id 7590082829145784578 \
  --fields-file ./new_fields.json
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--id <DATASET_ID>`：数据集 id（必填）
- `--fields '<json_array>'`：新字段定义的 JSON 数组字符串
  - 建议用单引号包裹，避免 shell 对 `"`、`{}` 等字符转义
  - 示例：`'[{"name":"score","contentType":1,"schemaKey":1}]'`
- `--fields-file <path>`：从文件读取字段定义（文件内容必须是 JSON array）
  - 适合字段定义较大或包含复杂转义时使用
- `--fields` 和 `--fields-file` 二选一，必须提供其一

#### 约束

- 这是追加操作（append-only），只能添加新列，不能修改或删除已有列
- 新字段的 `key` 必须为空（由系统自动分配）
- 字段 `name` 不能与已有列重名
- `contentType` 仅支持 1（Text）和 100（MultiPart）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `dataset_append_schema_<id>.json`
