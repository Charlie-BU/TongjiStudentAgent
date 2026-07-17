# fornax-cli skill

Skill 资源相关命令，包含创建、列表查询、详情查询、版本提交、提交记录查询与批量查询。

## skill create

创建一个 Skill。

### 用法

```bash
fornax-cli skill create [选项]
```

### zip_data

- `--zip-file`：读取本地 zip 文件并发送其 base64 内容

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：
  - 优先：`skill_<skill_id>.json`（接口返回 `skill_id` 时）
  - 否则：`skill_create_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill create --name demo --skill-key demo_key --description "demo" --zip-file ./demo.zip
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：Skill 名称（必填）
- `--skill-key <key>`：Skill key（必填）
- `--description <text>`：Skill 描述（可选）
- `--zip-file <path>`：zip 文件路径（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入对应输出文件（见上）

## skill get

按 skill-id 获取 Skill 详情。

### 用法

```bash
fornax-cli skill get [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_<skill_id>.json`

### 示例

```bash
fornax-cli skill get --skill-id 123
fornax-cli skill get --skill-id 123 --with-commit --commit-version 1.0.0
fornax-cli skill get --skill-id 123 -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-id <id>`：Skill ID（必填）
- `--with-commit`：是否包含 commit 信息（可选）
- `--commit-version <ver>`：指定查询的 commit 版本（可选）
- `--with-public-draft`：是否包含 public draft（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_<skill_id>.json`

## skill list

分页列出 Skills，并可按条件过滤。

### 用法

```bash
fornax-cli skill list [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_list_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill list
fornax-cli skill list --keyword demo --page-no 1 --page-size 20
fornax-cli skill list --filter-uncommitted
fornax-cli skill list -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--keyword <kw>`：关键字过滤（可选）
- `--created-by <id>`：创建者过滤（可选）
- `--source <source>`：来源过滤（可选）
- `--filter-uncommitted`：是否过滤未提交版本的 skill（可选）
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页数量；默认 20；范围 1~100
- `--asc`：是否升序排序（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_list_<YYYYMMDDHHMMSS>.json`

## skill commit

提交一个 Skill 版本。

### 用法

```bash
fornax-cli skill commit [选项]
```

### zip_data

- `--zip-file`：读取本地 zip 文件并发送其 base64 内容

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_commit_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill commit --skill-id 123 --commit-version 1.0.0 --commit-description "init" --zip-file ./demo.zip
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-id <id>`：Skill ID（必填）
- `--commit-version <ver>`：提交版本号（必填）
- `--commit-description <text>`：提交说明（可选）
- `--zip-file <path>`：zip 文件路径（必填）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_commit_<YYYYMMDDHHMMSS>.json`

## skill list-commits

分页列出某个 Skill 的提交记录。

### 用法

```bash
fornax-cli skill list-commits [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_list_commits_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill list-commits --skill-id 123 --page-size 20
fornax-cli skill list-commits --skill-id 123 --page-size 20 --page-token <TOKEN>
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-id <id>`：Skill ID（必填）
- `--page-size <N>`：每页数量（必填，且 > 0）；默认 20
- `--page-token <token>`：翻页 token（可选）
- `--asc`：是否升序排序（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_list_commits_<YYYYMMDDHHMMSS>.json`

## skill save-detail

保存 Skill 的详情（携带 zip_data）。

### 用法

```bash
fornax-cli skill save-detail [选项]
```

### zip_data

- `--zip-file`：读取本地 zip 文件并发送其 base64 内容

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_save_detail_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill save-detail --skill-id 123 --zip-file ./demo.zip
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-id <id>`：Skill ID（必填）
- `--zip-file <path>`：zip 文件路径（必填）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_save_detail_<YYYYMMDDHHMMSS>.json`

## skill batch-get-by-id

按 skill-id 批量查询 Skills。

### 用法

```bash
fornax-cli skill batch-get-by-id [选项]
```

### 输入（选择其一）

- `--skill-id`：重复传入多个 skill-id；可配合 `--version` 指定统一版本
- `--skill-query-json`：直接传 JSON 数组，或用 `@file` 从文件读取

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_batch_get_by_id_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill batch-get-by-id --skill-id 123 --skill-id 456
fornax-cli skill batch-get-by-id --skill-id 123 --version 1.0.0
fornax-cli skill batch-get-by-id --skill-query-json '[{"skill_id":"123","version":"1.0.0"}]'
fornax-cli skill batch-get-by-id --skill-query-json @./queries.json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-id <id>`：Skill ID（可重复）
- `--version <ver>`：Skill version（可选；仅在使用 `--skill-id` 输入时生效）
- `--skill-query-json <json|@file>`：JSON 数组（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_batch_get_by_id_<YYYYMMDDHHMMSS>.json`

## skill batch-get-by-key

按 skill-key 批量查询 Skills。

### 用法

```bash
fornax-cli skill batch-get-by-key [选项]
```

### 输入（选择其一）

- `--skill-key`：重复传入多个 skill-key；可配合 `--version` 指定统一版本
- `--skill-query-json`：直接传 JSON 数组，或用 `@file` 从文件读取

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`skill_batch_get_by_key_<YYYYMMDDHHMMSS>.json`

### 示例

```bash
fornax-cli skill batch-get-by-key --skill-key demo_key --version 1.0.0
fornax-cli skill batch-get-by-key --skill-query-json '[{"skill_key":"demo_key","version":"1.0.0"}]'
fornax-cli skill batch-get-by-key --skill-query-json @./queries.json -o ./out
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--skill-key <key>`：Skill key（可重复）
- `--version <ver>`：Skill version（可选；仅在使用 `--skill-key` 输入时生效）
- `--skill-query-json <json|@file>`：JSON 数组（可选）

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `skill_batch_get_by_key_<YYYYMMDDHHMMSS>.json`

## skill install

根据 skill-key 批量拉取 Skill 的 zip 包（从 `tos_url` 下载），并解压到指定目录。

当不传 `--dir` 时，会交互式选择要安装到哪个 Agent（含"全局安装"选项），并下载到对应目录。

如果所选 Agent 的一级目录不存在（例如 `~/.openclaw` 不存在），会提示你确认安装目录；你可以用 `--dir` 手动指定安装目录。

交互选择支持 ↑/↓ 移动，回车确认。

- `~/.openclaw/skills/`
- `~/.coco/skills/`
- `~/.trae/skills/`
- `~/.claude/skills/`
- `~/.cursor/skills/`
- `~/.codex/skills/`
- `~/.gemini/skills/`

解压目录说明：

- 不会额外创建 `/<skill_key>/<version>` 目录层级；直接解压到所选 Agent 的 skills 目录（或 `--dir` 指定目录）。
- 如果 skill 来源是 `ai_paas` 且 `md_info.name` 不为空，会额外包一层目录：`/<md_info.name>/`。

### 用法

```bash
fornax-cli skill install [skill-key ...] [选项]
```

### 输入（选择其一）

- 位置参数：直接传入一个或多个 skill-key
- `--skill-query-json`：直接传 JSON 数组，或用 `@file` 从文件读取

### 示例

```bash
fornax-cli skill install demo_key
fornax-cli skill install demo_key --dir ./out --z-ti-token '<TOKEN>'
fornax-cli skill install --skill-query-json '[{"skill_key":"demo_key","version":"1.0.0"}]'
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--dir <DIR>`：下载并解压到该目录（可选）
- `--z-ti-token <TOKEN>`：下载 `tos_url` 时使用的 token（header: `X-Zti-Token`）（可选）
- `--skill-query-json <json|@file>`：JSON 数组（可选）
