# fornax-cli prompt

Prompt 资源相关命令，包含：列表查询、按 key 获取详情（需已发布）、创建、删除、按 id 获取详情、草稿管理、提交管理和发布管理。

## 基础命令

### prompt list

分页列出 Prompt，并可按关键字前缀过滤。

#### 用法

```bash
fornax-cli prompt list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_list_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli prompt list
fornax-cli prompt list --keyword demo
fornax-cli prompt list --page-no 2 --page-size 50
fornax-cli prompt list -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--keyword <prefix>`：按 prompt 的 name/key 前缀过滤；不传则不过滤
- `--page-no <N>`：页码，从 1 开始；默认 1
- `--page-size <N>`：每页数量；默认 20；建议范围 1\~200（过大可能导致接口返回慢或失败）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_list_<YYYYMMDDHHMMSS>.json`

### prompt get-by-key

按 key 获取 Prompt 详情，可选指定提交版本，并可包含草稿和提交信息。

#### 用法

```bash
fornax-cli prompt get-by-key [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_<key>.json`

#### 示例

```bash
fornax-cli prompt get-by-key --key demo_key
fornax-cli prompt get-by-key --key demo_key --version 1.0.0
fornax-cli prompt get-by-key --key demo_key --with-draft
fornax-cli prompt get-by-key --key demo_key --with-commit
fornax-cli prompt get-by-key --key demo_key --with-commit --commit-version 1.0.0
fornax-cli prompt get-by-key --key demo_key -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--key <key>`：Prompt key（必填，全局唯一）。使用三段式点分格式：`<segment1>.<segment2>.<segment3>`（如 `project.module.name`），仅支持英文字母、数字、下划线和点；通常是你在 `prompt list` 里看到的 key
- `--version <ver>`：可选，指定 prompt 提交版本（示例：`1.0.0`）；不传则由服务端返回默认/最新版本
- `--with-draft`：包含草稿信息
- `--with-commit`：包含提交信息
- `--commit-version <ver>`：指定要包含的提交版本（需配合 `--with-commit` 使用）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_<key>.json`

### prompt create

创建一个新的 Prompt。

#### 用法

```bash
fornax-cli prompt create [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_<prompt_id>.json`

#### 示例

```bash
fornax-cli prompt create --name demo --key demo_key
fornax-cli prompt create --name demo --key demo_key --description "A demo prompt"
fornax-cli prompt create --name demo --key demo_key --type normal -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--name <name>`：Prompt 名称（必填）
- `--key <key>`：Prompt key（必填，全局唯一）。使用三段式点分格式：`<segment1>.<segment2>.<segment3>`（如 `project.module.name`），仅支持英文字母、数字、下划线和点
- `--description <text>`：描述（可选）
- `--type <type>`：Prompt 类型（可选）：`normal`、`snippet`

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_<prompt_id>.json`

### prompt delete

按 id 删除一个 Prompt。

#### 用法

```bash
fornax-cli prompt delete [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_delete_<prompt_id>.json`

#### 示例

```bash
fornax-cli prompt delete --prompt-id <ID>
fornax-cli prompt delete --prompt-id <ID> -y
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `-y, --yes`：跳过二次确认（用于脚本/CI；谨慎使用，避免误删）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_delete_<prompt_id>.json`

### prompt get-by-id

按 id 获取 Prompt 详情，可选包含草稿和提交信息。

#### 用法

```bash
fornax-cli prompt get-by-id [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_<prompt_id>.json`

#### 示例

```bash
fornax-cli prompt get-by-id --prompt-id <ID>
fornax-cli prompt get-by-id --prompt-id <ID> --with-draft
fornax-cli prompt get-by-id --prompt-id <ID> --with-commit --commit-version 1.0.0
fornax-cli prompt get-by-id --prompt-id <ID> -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--with-draft`：包含草稿信息
- `--with-commit`：包含提交信息
- `--commit-version <ver>`：指定要包含的提交版本（需配合 `--with-commit` 使用）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_<prompt_id>.json`

## 草稿（Draft）

### prompt draft save

保存 Prompt 草稿。

> **重要**：保存模式会使用入参全量覆盖草稿，而非增量更新。每次保存时必须提供完整的草稿内容，不能只传递修改的部分。多轮编辑更新过程中，请确保 messages、variable\_defs、tools、model\_config 等所有字段都包含在内，以免丢失信息。

#### 用法

```bash
fornax-cli prompt draft save [选项]
```

#### 草稿内容（Draft，选择其一）

- `--draft`：JSON object 字符串
- `--draft-file`：JSON 文件路径（内容必须是 JSON object）

草稿 JSON 对象代表 PromptDraft。最简结构（首次保存，无需 base\_version）：

```json
{
  "detail": {
    "prompt_template": {
      "template_type": "normal",
      "messages": [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello, {{name}}!"}
      ],
      "variable_defs": [
        {"key": "name", "type": "string"}
      ]
    }
  }
}
```

完整结构（含所有可选部分）：

```json
{
  "detail": {
    "prompt_template": {
      "template_type": "normal",
      "messages": [
        {"role": "system", "content": "你是万能机器人，你的名字叫{{name}}"},
        {"role": "placeholder", "content": "history_message"},
        {"role": "user", "content": "", "parts": [
          {"type": "multi_part_variable", "text": "image"},
          {"type": "text", "text": " 分析一下图片"}
        ]},
        {"role": "assistant", "content": "this is a assistant"}
      ],
      "variable_defs": [
        {"key": "name", "type": "string"},
        {"key": "image", "type": "multi_part"},
        {"key": "history_message", "type": "placeholder"}
      ],
      "has_snippet": false
    },
    "tools": [
      {"type": "function", "function": {
        "name": "get_weather",
        "description": "Determine weather in my location",
        "parameters": "{\"type\":\"object\",\"properties\":{\"location\":{\"type\":\"string\",\"description\":\"The city and state e.g. San Francisco, CA\"},\"unit\":{\"type\":\"string\",\"enum\":[\"c\",\"f\"]}},\"required\":[\"location\"]}"
      }}
    ],
    "tool_call_config": {"tool_choice": "none"},
    "model_config": {
      "frequency_penalty": 0,
      "max_tokens": 4096,
      "model_id": "1756106585",
      "temperature": 1,
      "top_p": 0.7
    },
    "skill_execute_config": {
      "skill_combine": [
        {"skill_id": "123", "version": "v1", "name": "my_skill", "description": "desc", "source": "custom", "is_deleted": false, "skill_key": "my_skill_key"}
      ],
      "sandbox_config": {
        "sandbox_psm": "my.sandbox.psm",
        "session_id": "sess_001",
        "name": "sandbox_name",
        "description": "sandbox desc",
        "region": "cn-north",
        "type": "default",
        "sandbox_id": "sbx_001",
        "owner": "user1",
        "is_deleted": false,
        "resource_limit": {"cpu_milli": 2000, "mem_mb": 4096},
        "session_limit": {"max": 10, "min": 1}
      }
    }
  },
  "draft_info": {
    "base_version": "0.0.7"
  }
}
```

**Messages**：

- `role`：`system`、`user`、`assistant`、`placeholder`
- `content`：文本内容，支持 `{{variable}}` 模板语法
- `parts`：多模态内容（`multi_part_variable`、`text`、`image_url`、`video_url`）

**Variable definitions（variable\_defs）**：

- `type`：`string`、`multi_part`、`placeholder`

**Tools**：

- `type`：`"function"`，包含 `name`、`description` 和 JSON Schema 格式的 `parameters`

**Model config（model\_config，可选）**：

- `model_id`：必须是在 Fornax 平台上注册的 model ID
- `max_tokens`：生成的最大 token 数
- `temperature`：采样温度（0\~2）
- `top_p`：核采样参数（0\~1）
- `frequency_penalty`：频率惩罚（-2\~2）

**Skill execute config（skill\_execute\_config，可选）**：

- `skill_combine`：绑定的 skill 列表
  - `skill_id`：skill ID（字符串）
  - `version`：skill 版本
  - `name`：skill 名称
  - `description`：skill 描述
  - `source`：skill 来源
  - `is_deleted`：是否已删除
  - `skill_key`：skill 唯一标识
- `sandbox_config`：沙箱配置
  - `sandbox_psm`：沙箱 PSM 地址
  - `session_id`：会话 ID
  - `name`：沙箱名称
  - `description`：沙箱描述
  - `region`：区域
  - `type`：沙箱类型
  - `sandbox_id`：沙箱 ID
  - `owner`：拥有者
  - `is_deleted`：是否已删除
  - `resource_limit`：资源限制（`cpu_milli`：CPU 毫核，`mem_mb`：内存 MB）
  - `session_limit`：会话限制（`max`：最大值，`min`：最小值）

**draft\_info（可选）**：

- `base_version`：草稿基于的 commit 版本号；首次保存草稿时无需传递。当基于已有提交版本进行编辑时，设置为该版本号（例如 `"0.0.7"`），以便平台追踪编辑来源

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_draft_save_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
# 简单草稿（内联 JSON）
fornax-cli prompt draft save --prompt-id <ID> --draft '{"detail":{"prompt_template":{"template_type":"normal","messages":[{"role":"system","content":"You are a helpful assistant."}]}}}'

# 从文件加载复杂草稿（含多模态变量、工具、base_version 等）
fornax-cli prompt draft save --prompt-id <ID> --draft-file ./draft.json

# 保存并输出到目录
fornax-cli prompt draft save --prompt-id <ID> --draft-file ./draft.json -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--draft '<json_object>'`：草稿的 JSON 对象字符串
  - 建议用单引号包裹，避免 shell 对 `"`、`{}` 等字符转义
- `--draft-file <path>`：从文件读取草稿（文件内容必须是 JSON object）
  - 适合草稿内容较大或包含复杂转义时使用

注意：`--draft` 与 `--draft-file` 必选其一。

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_draft_save_<YYYYMMDDHHMMSS>.json`

### prompt draft commit

将 Prompt 草稿提交为新版本。

#### 用法

```bash
fornax-cli prompt draft commit [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_draft_commit_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli prompt draft commit --prompt-id <ID> --version 1.0.0
fornax-cli prompt draft commit --prompt-id <ID> --version 1.0.0 --description "Initial version"
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--version <ver>`：提交版本号（必填），格式：`MAJOR.MINOR.PATCH`（如 `0.0.1`、`1.0.0`、`2.1.3`）
- `--description <text>`：提交描述（可选）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_draft_commit_<YYYYMMDDHHMMSS>.json`

## 提交（Commit）

### prompt commit list

分页列出 Prompt 的提交历史。

#### 用法

```bash
fornax-cli prompt commit list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_commit_list_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli prompt commit list --prompt-id <ID>
fornax-cli prompt commit list --prompt-id <ID> --page-size 50
fornax-cli prompt commit list --prompt-id <ID> --with-detail
fornax-cli prompt commit list --prompt-id <ID> --page-token <TOKEN> -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--page-size <N>`：每页条数；默认 20
- `--page-token <token>`：分页 token（可选）；通常把上一次 list 返回的 page\_token 原样传回，用于继续拉取下一页
- `--with-detail`：包含提交详情

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_commit_list_<YYYYMMDDHHMMSS>.json`

## 发布（Release）

### prompt release create

为 Prompt 的某个提交版本创建发布任务。

#### 用法

```bash
fornax-cli prompt release create [选项]
```

#### 发布配置（Release Config，必填，选择其一）

- `--release-config`：JSON object 字符串
- `--release-config-file`：JSON 文件路径（内容必须是 JSON object）

支持的 release config 字段：

```json
{
  "approval_escape": true,
  "gray_release_strategy": "none"
}
```

`gray_release_strategy` 可选值：

- `none`：不开启灰度（默认）
- `instance_gray_release`：开启实例级小流量灰度发布

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_release_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli prompt release create --prompt-id <ID> --commit-version 1.0.0 --env online --release-config '{"approval_escape":true}'
fornax-cli prompt release create --prompt-id <ID> --commit-version 1.0.0 --env online --feature default --release-config-file ./release_config.json
fornax-cli prompt release create --prompt-id <ID> --commit-version 1.0.0 --env boe --release-config '{"approval_escape":false}' -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--commit-version <ver>`：要发布的提交版本（必填），示例：`1.0.0`
- `--env <name>`：环境（必填）：`boe` 或 `online`（online 包含 ppe 环境）
- `--release-config '<json_object>'`：发布配置 JSON 对象字符串（必填，与 `--release-config-file` 二选一）。支持字段：`approval_escape`（bool）、`gray_release_strategy`（string: `none` | `instance_gray_release`）
- `--release-config-file <path>`：发布配置 JSON 文件路径（与 `--release-config` 二选一）
- `--feature <name>`：Feature 名称（可选）
- `--label <label>`：标签（可选）
- `--comment <text>`：评论（可选）

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT），pretty 模式下会显示工单 ID、工单 URL（release\_task\_url）以及校验结果（check\_result）
- 传 `-o <DIR>`：写入 `prompt_release_<YYYYMMDDHHMMSS>.json`

### prompt release list

列出 Prompt 的发布信息，支持多种过滤条件。

#### 用法

```bash
fornax-cli prompt release list [选项]
```

#### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`prompt_release_list_<YYYYMMDDHHMMSS>.json`

#### 示例

```bash
fornax-cli prompt release list --prompt-id <ID>
fornax-cli prompt release list --prompt-id <ID> --env online --status online
fornax-cli prompt release list --prompt-id <ID> --version-like "1.0" --page-size 50 -o ./out
```

#### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--prompt-id <ID>`：Prompt id（必填）
- `--version <ver>`：按精确版本过滤
- `--feature <name>`：按 feature 过滤
- `--label <label>`：按标签过滤
- `--version-like <pattern>`：按版本前缀/模式过滤
- `--env <name>`：按环境过滤：`boe` 或 `online`（online 包含 ppe 环境）
- `--status <name>`：按状态过滤：`online`、`offline`、`in_gray`、`canary`、`single_dc`
- `--cursor <N>`：分页游标（首次查询不传，翻页时传入）
- `--page-size <N>`：每页条数；默认 20

#### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `prompt_release_list_<YYYYMMDDHHMMSS>.json`

