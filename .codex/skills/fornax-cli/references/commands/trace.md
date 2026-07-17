# fornax-cli trace / span

Trace 和 Span 资源相关命令，包含 trace 的单条查询、列表拉取，以及 span 的直接搜索。

## 标识概念（trace-id / span-id / log-id）

- Trace ID（trace-id）：一次端到端请求/调用链路的全局标识，用于把同一次请求产生的多个 span 串起来。一般对应一轮用户对话和agent返回。
  - 示例：`<TRACE_ID>`（示例形态：`4bf92f3577b34da6a3ce929d0e0e4736`，以实际返回为准）
- Span ID（span-id）：trace 内单个 span 的标识（例如一次模型调用、一次工具调用等）。
  - 示例：`<SPAN_ID>`（示例形态：`00f067aa0ba902b7`，以实际返回为准）
- Log ID（log-id）：服务端一次请求的唯一标识。一般一个log-id对应一个trace-id。
  - 示例：`<LOG_ID>`（示例形态：`20260316105542BFA43CCB10CB8AA04E76`或`02177372567743326050340cd512a0076cfd91782472ff3ae33c4`，以实际返回为准）

## trace get

通过 trace-id 或 log-id 获取单个 trace 的所有 span 数据。

### 用法

```bash
fornax-cli trace get [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`trace_<traceID or logID>.json`（`--tree` 模式下为 `trace_<ID>_tree.json`）
- 文件去重：如果输出目录中已存在同名文件，会自动在文件名后追加时间戳（如 `trace_<ID>_20260423150000.json`），不会覆盖已有文件

### 关键约束

- `--trace-id` 与 `--log-id` 互斥，必须二选一
- `--start-ms` 必须与 `--end-ms` 同时使用

### 示例

```bash
fornax-cli trace get --trace-id <TRACE_ID>
fornax-cli trace get --log-id <LOG_ID>
fornax-cli trace get --trace-id <TRACE_ID> --span-id <SPAN_ID_1>,<SPAN_ID_2>
fornax-cli trace get --trace-id <TRACE_ID> --tree
fornax-cli trace get --trace-id <TRACE_ID> --since 2026-03-17T00:00:00+08:00 --until 2026-03-17T01:00:00+08:00 -o ./out
```

### 参数说明

#### Trace 标识（必选其一）

- `--trace-id <TRACE_ID>`：Trace ID；与 `--log-id` 互斥
- `--log-id <LOG_ID>`：Log ID；与 `--trace-id` 互斥

#### Span 过滤

- `--span-id <ID1>,<ID2>,...`：可选，按 span ID 过滤。支持多个 ID 逗号分隔（例如 `--span-id id1,id2,id3`）
- `--span-filter-expr "<expr>"`：详见 filter-expr 详解（用于进一步缩小结果）
- `--tree`：一次性获取所有 span（无分页）。注意：tree 模式**不返回** span 的 input/output 字段。与 `--span-id` 可组合使用

#### 其他

- `--platform-type <TYPE>`：可选，平台类型。可选值：`open_api`（默认）、`inner_doubao`（豆包）、`inner_doubao_aw`（豆包白名单）
- `-h, --help`：显示本子命令帮助信息并退出

### 输出与 -o
返回数据较多、复杂分析场景，建议使用 `-o` 输出到目录。

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `trace_<traceID or logID>.json`
- 输出格式与 span 排列：
  - `--format raw`：每个 span 一行紧凑 JSON（NDJSON / JSON Lines 格式）
  - `--format json`：完整 JSON 数组（缩进格式）
  - `-o` 导出文件时格式规则相同

## trace list

列出 trace 列表，并下载每条 trace 的 spans。

### 用法

```bash
fornax-cli trace list [选项]
```

### 输出文件
返回数据较多、复杂分析场景，建议使用 `-o` 输出到目录。

- `-o <DIR>`：指定输出目录
- 每条 trace 保存为：`trace_<traceID>.json`
- 输出格式与 span 排列：
  - `--format raw`：每个 span 一行紧凑 JSON（NDJSON / JSON Lines 格式）
  - `--format json`：完整 JSON 数组（缩进格式）
  - `-o` 导出文件时格式规则相同

### 示例

```bash
fornax-cli trace list --last-n-minutes 60 --page-size 5
fornax-cli trace list --trace-filter-expr "duration>1000" --span-filter-expr "span_type='model'" -o ./out
```

### 参数说明

#### 数量控制

- `--page-size <N>`：最多拉取多少条 trace；默认 1

#### 过滤表达式

- `--trace-filter-expr "<expr>"`：详见 filter-expr 详解（基于 root span 属性筛选 trace）
- `--span-filter-expr "<expr>"`：详见 filter-expr 详解（在确定 trace 后，过滤 trace 下返回的 spans）

#### 其他

- `--platform-type <TYPE>`：可选，平台类型。可选值：`open_api`（默认）、`inner_doubao`（豆包）、`inner_doubao_aw`（豆包白名单）
- `-h, --help`：显示本子命令帮助信息并退出

## span list

直接从 span 索引中搜索和列出 span，适合 span 级别的筛选和分析。

与 `trace list` 的区别：`trace list` 先搜 root span 获取 trace ID 列表，再逐条拉取完整 trace 树；`span list` 直接返回符合条件的 span，无需展开整棵 trace 树。

### 用法

```bash
fornax-cli span list [选项]
```

### 输出文件

- `-o <DIR>`：指定输出目录
- 文件名：`spans_list_<timestamp>.json`

### 示例

```bash
# 最近 1 小时所有 model 类型 span
fornax-cli span list --last-n-minutes 60 --span-filter-expr "span_type='model'"

# duration > 2000ms，50 条，JSON 格式
fornax-cli span list --span-filter-expr "duration > 2000" --page-size 50 --format json

# 指定时间范围，写文件
fornax-cli span list --since 2026-04-01T00:00:00+08:00 --until 2026-04-01T12:00:00+08:00 -o ./out

# 组合条件
fornax-cli span list --span-filter-expr "span_type='model' AND input_tokens > 100" --page-size 100

# 分页：第一页
fornax-cli span list --span-filter-expr "span_type='model'" --page-size 20 --start-ms 1743436800000 --end-ms 1743523200000

# 分页：使用上一页返回的 page token 获取下一页（注意：分页时建议使用 --start-ms/--end-ms 保持时间窗口一致）
fornax-cli span list --span-filter-expr "span_type='model'" --page-size 20 --start-ms 1743436800000 --end-ms 1743523200000 --page-token <TOKEN_FROM_PREVIOUS>
```

### 参数说明

#### 数量控制与分页

- `--page-size <N>`：最大返回 span 数量；默认 20
- `--page-token <TOKEN>`：分页 token。使用上一次响应返回的 next\_page\_token 获取下一页

#### 过滤表达式

- `--span-filter-expr "<expr>"`：Span 过滤表达式（SQL WHERE 子集语法）。详见下文"过滤表达式（filter-expr）详解"

#### 其他

- `--span-list-type <TYPE>`：可选，span 列表类型。可选值：`all_span`（默认）、`root_span`（仅根 span）、`llm_span`（仅 LLM 相关 span）
- `--platform-type <TYPE>`：可选，平台类型。可选值：`open_api`（默认）、`inner_doubao`（豆包）、`inner_doubao_aw`（豆包白名单）
- `-h, --help`：显示本子命令帮助信息并退出

### 输出与 -o

- 不传 `-o`：直接输出到标准输出（STDOUT）
- 传 `-o <DIR>`：写入 `spans_list_<timestamp>.json`
- 文件去重：如果输出目录中已存在同名文件，会自动追加时间戳，不会覆盖已有文件
- 输出格式：
  - `--format raw`：每个 span 一行紧凑 JSON（NDJSON / JSON Lines 格式）
  - `--format json`：完整 JSON 数组（缩进格式）
  - `--format pretty`（默认）：表格格式，展示 trace\_id, span\_id, span\_name, span\_type, input, output

### 分页说明

当结果超过 `--page-size` 限制时，命令会输出 `next_page_token` 提示信息和示例命令。

**重要**：分页时建议使用 `--start-ms/--end-ms`（绝对时间戳）而非 `--last-n-minutes`（相对时间），以保证跨页查询的时间窗口一致。如果使用 `--last-n-minutes`，每次执行时 "now" 会变化，可能导致分页结果不连续。

## 时间窗口（可选，三选一）

以上所有命令（trace get / trace list / span list）共享相同的时间窗口参数规则：

不传时间窗口时：`trace get` 由服务端默认行为决定（建议显式传入）；`trace list` 和 `span list` 默认查询最近 24 小时。

- `--last-n-minutes <N>`：查询最近 N 分钟内的 trace/span 数据（示例：`60`）
- 开始结束时间
  - `--since <ISO8601>`：开始时间（ISO 8601，示例：`2026-03-17T00:00:00+08:00`）；必须与 `--until` 同时提供
  - `--until <ISO8601>`：结束时间（ISO 8601，示例：`2026-03-17T01:00:00+08:00`）；必须与 `--since` 同时提供
- 开始结束时间戳
  - `--start-ms <ms>`：开始时间（Unix 毫秒时间戳）；必须与 `--end-ms` 同时提供
  - `--end-ms <ms>`：结束时间（Unix 毫秒时间戳）；必须与 `--start-ms` 同时提供


## 过滤表达式（filter-expr）详解

`--trace-filter-expr` 与 `--span-filter-expr` 都使用同一套过滤表达式语法（SQL WHERE 子集），但它们的作用范围不同。

### 两类 filter-expr 的作用范围

- `--trace-filter-expr`：用于筛选 trace（trace list 场景）。表达式会基于每条 trace 的 root span 属性进行匹配；匹配成功的 trace 才会被返回/下载。
- `--span-filter-expr`：用于过滤 span。在 trace get / trace list 中，当 trace 已经被确定后，表达式会对 trace 下的 spans 做过滤。在 span list 中，表达式直接用于搜索 span 索引。

经验规则：

- 想"筛出哪些 trace" → 用 `--trace-filter-expr`
- 想"同一个 trace 里只看部分 spans" → 用 `--span-filter-expr`（trace get / trace list）
- 想"直接搜索满足条件的 span" → 用 `--span-filter-expr`（span list）

### 字段（field）

表达式中的字段名为标识符（identifier）。支持常见字段（Common fields）以及自定义字段（tags）。

常见字段（Common fields）：

- logid, trace\_id, span\_name, span\_type（常用枚举：model, tool）, psm, duration, input, output, latency\_first\_resp, message\_id, psm\_env, status\_code, thread\_id, user\_id, deployment\_env（boe环境：boe，线上环境：!= boe）, input\_tokens, output\_tokens, tokens

自定义字段（tags）：可以直接用字段名引用，例如 `my_tag = 'foo'`。如果字段名包含特殊字符（如 `-`），请使用反引号包裹，例如 `` `my-tag` = 'foo' ``。

### 支持的运算符

支持以下运算符（左侧必须是字段名；右侧必须是字面量或字面量列表）：

- 比较：`=`、`!=`、`>`、`>=`、`<`、`<=`
- 集合：`IN (...)`、`NOT IN (...)`（列表不能为空）
- 匹配：`LIKE`、`NOT LIKE`（用于模糊匹配，不支持%通配符，默认行为为`LIKE '%<value>%'`）
- 空值判断：`IS NULL`、`IS NOT NULL`
- 逻辑连接与分组：`AND`、`OR`、`(...)`（支持括号；优先级遵循 SQL：`AND` 高于 `OR`）

不支持的常见写法：

- 通用 `NOT <expr>`（仅支持 `NOT IN`、`NOT LIKE`、`IS NOT NULL` 这种形态）
- 计算表达式或函数参与比较（例如 `duration/1000 > 1`、`len(input) > 0` 等）

### 值类型与类型推断

服务端表达式的值类型推断规则如下：

- 显式类型标注优先：
  - `string(field)` / `long(field)` / `double(field)` / `bool(field)`
- 否则，内置字段会使用固定类型：
  - `trace_id`、`logid`、`span_id`、`span_name`、`span_type`、`psm`、`status` → `string`
  - `latency_first_resp`、`duration`、`input_tokens`、`output_tokens`、`tokens` → `long`
- 其余字段（包括 tags）按右侧"字面量值"推断（`IN (...)` 会综合列表里所有值）：
  - 只要出现字符串字面量（如 `'abc'`）→ `string`
  - 否则出现浮点/小数（如 `1.5`）→ `double`
  - 否则出现整数（如 `10`）→ `long`
  - 否则出现布尔（`true/false`）→ `bool`
  - 兜底 → `string`

注意：

- 字符串字面量建议使用单引号，例如 `psm = 'demo'`；在 shell 中通常用双引号包住整个表达式：`--span-filter-expr "psm = 'demo' and duration > 1000"`。
- 需要判断空值请使用 `IS NULL` / `IS NOT NULL`。`field = NULL` 不等价于空值判断，会被当作字符串 `"null"` 参与比较。

### 示例

```bash
fornax-cli trace list --trace-filter-expr "psm = 'demo' and duration >= 1000" --page-size 5
fornax-cli trace list --span-filter-expr "span_type = 'model' and latency_first_resp < 500" --page-size 5
fornax-cli trace list --span-filter-expr "status_code in (200, 500) and deployment_env != 'prod'" --page-size 5
fornax-cli trace list --span-filter-expr "my_tag = 'foo' and `my-tag` like 'bar%'" --page-size 5
fornax-cli span list --span-filter-expr "span_type='model' AND duration > 1000" --page-size 20
fornax-cli span list --span-filter-expr "input_tokens > 100" --last-n-minutes 60 --format json
```
