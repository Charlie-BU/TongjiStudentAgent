# fornax-cli synthesis

智能合成（Data Synthesis）相关命令，包含：创建并运行合成任务、终止合成任务、列出合成任务。

## 创建并运行合成任务

### synthesis create-and-run

一步完成合成任务的创建与启动，支持异步、同步、试运行三种模式。

#### 用法

```bash
fornax-cli synthesis create-and-run [选项]
```

#### 执行模式

- **异步模式（默认）**：创建任务并在后台执行，立即返回 jobID。用 `synthesis list --job-ids <JOB_ID>` 跟踪进度。
- **同步模式（`--sync`）**：等待合成任务完成后再返回，耗时较长。
- **试运行模式（`--dry-run`）**：预览合成结果，不实际创建或运行任务。返回示例数据项，不产生 jobID。

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`synthesis_create_and_run_<jobID 或 dryrun>.json`

#### 示例

```bash
# 异步模式（默认）
fornax-cli synthesis create-and-run --sample-size 100 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}},{"schema":{"name":"answer"}}]'

# 试运行模式（预览不执行）
fornax-cli synthesis create-and-run --sample-size 10 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}}]' --dry-run

# 同步模式（等待完成）
fornax-cli synthesis create-and-run --sample-size 50 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}}]' --sync

# 合成到已有数据集
fornax-cli synthesis create-and-run --sample-size 100 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}}]' --target-dataset-id <DATASET_ID>

# 创建新的评测集作为合成目标
fornax-cli synthesis create-and-run --sample-size 100 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}}]' --target-dataset-category 4

# 无种子数据集（纯生成式合成）
fornax-cli synthesis create-and-run --sample-size 100 \
  --cols '[{"schema":{"name":"question"},"synthesisDescription":"generate diverse user queries"}]'

# 从文件读取列定义
fornax-cli synthesis create-and-run --sample-size 100 --seed-dataset-id <ID> --cols-file ./cols.json

# 指定 sourceScenario
fornax-cli synthesis create-and-run --sample-size 100 --seed-dataset-id <ID> \
  --cols '[{"schema":{"name":"question"}}]' --source-scenario "datasetlist"
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--sample-size <N>`：合成样本数量，取值 1–1000（必填）

##### 列定义（Cols，必填，选择其一）

列定义用于描述合成数据的列结构，必须是 JSON array。

- `--cols '<json_array>'`：直接传列定义的 JSON 数组字符串
  - 建议用单引号包裹，避免 shell 对 `"`、`{}` 等字符转义
  - 每个元素必须包含 `"schema"` 对象，其中至少有 `"name"` 字段
  - 可选字段：`"synthesisDescription"`（列描述）、`"SynthesisRequirement"`（风格/格式要求）
  - 示例：`'[{"schema":{"name":"question"},"synthesisDescription":"user query"},{"schema":{"name":"answer"},"SynthesisRequirement":"formal tone"}]'`
- `--cols-file <path>`：从文件读取列定义（文件内容必须是 JSON array，与 `--cols` 二选一）

##### 种子数据集

- `--seed-dataset-id <ID>`：种子数据集 ID（可选）。用作合成的参考基础数据。如不传，则进行纯生成式合成。
- `--seed-dataset-version-id <ID>`：种子数据集版本 ID（可选）。如不传，使用草稿版本。

##### 目标数据集配置

- `--target-dataset-id <ID>`：将合成结果写入已有数据集（可选）。与 `--target-dataset-category` 互斥。
- `--target-dataset-category <N>`：创建新的目标数据集（可选）：1 = 普通数据集（General），4 = 评测集（Evaluation）。与 `--target-dataset-id` 互斥。如两者都不指定，系统只会创建一个用户不可见的内部数据集。

##### 提示词相关参数

- `--eval-prompt-key <key>`：提示词 key（来自 Fornax 提示词平台），用于指导合成方向（可选）
- `--eval-prompt-version <ver>`：提示词版本（可选）。不传则使用最新版本。
- `--eval-target-type <type>`：目标类型，如被评测对象的类型（可选）

##### 其他参数

- `--description <text>`：合成任务的简要描述（可选）
- `--source-scenario <scenario>`：合成场景来源（可选），如 `datasetlist`、`evalset`、`dataset`。默认为 `default`。
- `--dry-run`：试运行模式，预览合成结果而不实际创建任务（可选）。试运行耗时较长，推荐搭配 `--timeout` 使用（如 `--timeout 5m`）。
- `--sync`：同步模式，等待合成完成后再返回（可选）。同步合成耗时较长，推荐搭配 `--timeout` 使用（如 `--timeout 5m`）。

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
  - 异步/同步模式：显示 jobID 和 outputDatasetID
  - 试运行模式：显示预览数据项
- 传 `-o <DIR>`：写入 `synthesis_create_and_run_<jobID 或 dryrun>.json`

## 终止合成任务

### synthesis terminate

终止正在运行的合成任务。

#### 用法

```bash
fornax-cli synthesis terminate [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`synthesis_terminate_<jobID>.json`

#### 示例

```bash
fornax-cli synthesis terminate --job-id <JOB_ID>
fornax-cli synthesis terminate --job-id <JOB_ID> -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--job-id <JOB_ID>`：合成任务 ID（必填）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `synthesis_terminate_<jobID>.json`

## 列出合成任务

### synthesis list

列出合成任务，支持多种过滤条件。可用 `--job-ids` 查询指定任务的实时进度（状态、已生成/总量），这是跟踪合成任务进度的推荐方式。

#### 用法

```bash
fornax-cli synthesis list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`synthesis_list_<timestamp>.json`

#### 示例

```bash
# 列出所有合成任务
fornax-cli synthesis list

# 查询指定任务的进度
fornax-cli synthesis list --job-ids <JOB_ID>

# 按种子数据集名称模糊搜索
fornax-cli synthesis list --search-words my_dataset

# 按场景过滤
fornax-cli synthesis list --scene 1

# 查询多个任务并输出到文件
fornax-cli synthesis list --job-ids id1,id2 -o ./out

# 按关联数据集过滤
fornax-cli synthesis list --related-dataset-id <DATASET_ID>
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--job-ids <id1,id2,...>`：逗号分隔的任务 ID 列表（可选）。传入后只返回这些任务的信息，可用于实时跟踪进度。
- `--search-words <text>`：按种子数据集名称模糊搜索（可选）
- `--scene <N>`：按合成场景过滤（可选），1 = 种子数据泛化
- `--related-dataset-id <ID>`：按关联数据集 ID 过滤（可选），包括种子数据集和输出数据集

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT），以表格形式展示任务列表（包含 job_id、status、seed_dataset、sample_size、progress、output_dataset_id、desc、start_time）
- 传 `-o <DIR>`：写入 `synthesis_list_<timestamp>.json`

#### 任务状态

| 状态码 | 含义 |
|--------|------|
| 1 | Pending（等待中） |
| 2 | Running（运行中） |
| 3 | Completed（已完成） |
| 4 | Failed（失败） |
| 5 | Terminated（已终止） |
