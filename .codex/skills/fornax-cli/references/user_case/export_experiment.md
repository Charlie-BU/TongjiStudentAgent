# Fornax 导出评测实验报告（端到端）

用于在本仓库内通过 CLI 一次性串起：**提交导出任务（CSV）→ 轮询导出状态 → 下载 CSV 报告（自动处理 URL 中文编码）**。

## 适用场景（何时调用）

- 实验已经跑完（`status=success/failed/terminated`），需要把结果落成 CSV 给非技术成员看
- 想把若干 evaluator 的 `score`/`reason`、target 输出列、性能指标、人工标注列**自由组合**导出
- 用脚本/CI 自动跑导出（结果只有几十秒到几分钟，适合 wait 后直接下载）

## 你需要从用户/上下文拿到的信息

- `<EXPERIMENT_ID>`：要导出的实验 id
- 想导出哪几列：
  - eval-set 字段 key（如 `input` / `expected_output` / 自定义 key）
  - target 输出列名（如 `actual_output` / `trajectory` / 自定义输出名）
  - target 性能指标列（如 `eval_target_total_latency` / `eval_target_total_tokens`）
  - evaluator 版本 id 列表（每个版本会导出 `score` + `reason` 两列）
  - 是否要 `weighted_score` 加权列（依赖实验上 `evaluator_id_version_list[*].score_weight`）
  - 是否要某些 tag-key 的人工标注列
- 下载路径（目录或完整文件名）

## 默认约定

- 当前仅支持 `--export-type CSV`
- 导出任务在后端是异步的，但通常**秒级**完成（实测瞬时 success）
- 签名链接的 `x-expires` 默认有效期约 2 小时；过期后 `expired=true`，需要重新触发 export 拿新链接

## 关键字段（detail 里能查到的、和导出直接相关的）

`experiment detail --experiment-id <EXP_ID> --format json` 输出的 `experiment` 对象里：

- `evaluator_id_version_list[].evaluator_version_id` → 直接给 `--evaluator-version-ids`
- `evaluator_id_version_list[].score_weight` → 决定 `--weighted-score` 列是否有意义
- `eval_set.current_version.evaluation_set_schema.field_schemas[].key` → 给 `--eval-set-fields`

> 老版本 detail 可能不返回 `evaluator_id_version_list`/`item_retry_num`/`expt_template_meta`。如果你只看到 `id/status/eval_set/evaluator_field_mapping/expt_stats` 这几项，说明后端 detail handler 还没补完，可以用 `--api loop` 或在网关层确认。

## 操作步骤（推荐按顺序执行）

### 0) 鉴权 + workspace（已配则跳过）

```bash
fornax-cli auth login
fornax-cli config select-workspace
```

### 1) 确认实验已进入终态

```bash
fornax-cli experiment detail \
  --experiment-id <EXPERIMENT_ID> \
  --format json | jq '.experiment | {id, status, expt_stats}'
```

只在 `status` 为 `success / failed / terminated` 时再继续；`pending / processing` 阶段也允许 export，但结果可能不完整。

### 2) 提交导出任务

最常用的列组合：eval-set 字段 + target 输出 + 评估器分数与原因 + 加权总分。

```bash
EXPORT_ID=$(fornax-cli experiment export \
  --experiment-id <EXPERIMENT_ID> \
  --export-type CSV \
  --eval-set-fields input,expected_output \
  --eval-target-outputs actual_output \
  --evaluator-version-ids <EV_VER_ID_1>,<EV_VER_ID_2> \
  --weighted-score \
  --format json | jq -r '.export_id')

echo "EXPORT_ID=$EXPORT_ID"
```

**列规格参数**（至少要选一项）：

| 参数 | 含义 | 示例 |
|---|---|---|
| `--eval-set-fields` | eval-set 字段 key（逗号分隔或重复） | `input,expected_output` |
| `--eval-target-outputs` | target 输出列名 | `actual_output` |
| `--metrics` | target 性能指标列 | `eval_target_total_latency,eval_target_total_tokens` |
| `--evaluator-version-ids` | evaluator 版本 id（每个 id 导 `score` + `reason`） | `7590075755941673730,...` |
| `--weighted-score` | 导出加权总分列 | （bool） |
| `--tag-key-ids` | 人工标注 TagKeyID | `9001,9002` |

也可以用 `--export-columns-file ./columns.json` 直接传完整 `ExptResultExportColumnSpec` JSON，结构见 `references/commands/experiment.md`。

### 3) 拉取导出记录并下载（推荐 `--download`，CLI 自动处理 URL 编码）

```bash
fornax-cli experiment export-record \
  --experiment-id <EXPERIMENT_ID> \
  --export-id "$EXPORT_ID" \
  --download ./out
```

`--download` 行为：

- 值是已存在目录：保存为 `<dir>/expt_<EXP_ID>_<EXPORT_ID>.csv`
- 值是文件路径：直接写入该路径（父目录必须存在）
- 状态非 `Success`、`expired=true`、`url` 为空时，**以非 0 退出码失败**（适合 CI gate）
- 自动对 URL path 里的非 ASCII 字符（如中文文件名）做 `%XX` 编码——直接 `curl` 不编码会被 CDN 拒成 403（`secure-time-check-md5-failed`）

### 4) 轮询模板（如果导出耗时较长或要做 CI gate）

```bash
EXP_ID=<EXPERIMENT_ID>
XID=<EXPORT_ID>

for i in $(seq 1 60); do
  resp=$(fornax-cli experiment export-record \
    --experiment-id "$EXP_ID" --export-id "$XID" --format json)
  status=$(echo "$resp" | jq -r '.expt_result_export_record.csv_export_status')
  echo "[$i] status=$status"
  case "$status" in
    Success)
      fornax-cli experiment export-record \
        --experiment-id "$EXP_ID" --export-id "$XID" \
        --download "./expt_${EXP_ID}.csv"
      exit 0;;
    Failed)
      echo "export failed:"; echo "$resp" | jq '.expt_result_export_record.error'
      exit 1;;
  esac
  sleep 5
done
echo "timeout waiting for export Success"
exit 1
```

### 5) 解析 CSV（结构）

CSV 列顺序（实际由后端决定）大致是：

```
ID, status, <eval-set-fields...>, <eval-target-outputs...>,
<metrics...>, <evaluator_name<version>>, <evaluator_name<version>_reason>,
<weightedScore>, <tag-key 标注列...>
```

例如：

```
ID,status,input,output,actual_output,相关性2251<0.0.1>,相关性2251<0.0.1>_reason,weightedScore
7577244608283803649,success,a,a,,1.00,输出内容与参考输出一致,1.00
```

注意：

- 文件以 UTF-8 BOM (`﻿`) 开头，Excel 直接打开能正确识别中文
- evaluator 列名形如 `<evaluator_name><<version>>`，再加 `_reason` 列
- 加权总分列名固定为 `weightedScore`，没勾 `--weighted-score` 就不会出现

## 常见排障

- 提交 export 报「at least one column-spec flag is required」：必须至少选一项列规格（`--eval-set-fields` / `--eval-target-outputs` / `--metrics` / `--evaluator-version-ids` / `--weighted-score` / `--tag-key-ids` 任一）
- 提交 export 报 400 `invalid request value, expected int`：`--export-id` 必须是数字字符串，别把 `<EXPORT_ID>` 占位符直接传进去
- `--download` 报「parent directory does not exist」：传文件路径时父目录必须先 `mkdir -p`；想自动用 `expt_<exp>_<xid>.csv` 命名直接传目录就行
- `--download` 不带 `-`o`时仍然会打印 record JSON 到 stdout，CSV 落盘信息写到 stderr，方便管道处理
- 状态 `Success` 但 `url` 是空字符串：极少见，通常是后端临时异常，重新发一次 `experiment export`
- 浏览器/wget 能下，curl 不能下：是中文 path 编码问题，**用 `--download` 走 CLI** 就行；如果非要用 curl，需要先对 URL path 段做 `urlencode`

## 与 submit_experiment 的关系

一个典型完整链路：

1. 按 [submit_experiment.md](./submit_experiment.md) 创建 eval-set + evaluator + experiment
2. `experiment detail` 等到终态
3. `experiment agg-results` 做效果/性能/成本 gate
4. **本文档：`experiment export` + `experiment export-record --download` 落 CSV 给非技术成员或归档**
