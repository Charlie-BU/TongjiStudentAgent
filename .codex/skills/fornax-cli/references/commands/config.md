# fornax-cli config

配置相关命令，用于管理 fornax-cli 使用的 `ak/sk/custom-region/endpoint` 等配置项。

配置文件加载优先级（高 → 低）：
1. 本地配置：`./.fornax-cli/config.yaml`（当前目录）
2. 全局配置：`~/.fornax-cli/config.yaml`（用户 home 目录）

如果两个配置文件同时存在，本地配置中的值会覆盖全局配置中的同名值。

## config set

向配置文件写入配置项。默认写入全局配置文件 `~/.fornax-cli/config.yaml`；使用 `--local` 时写入当前目录的 `./.fornax-cli/config.yaml`。

### 用法

```bash
fornax-cli config set <ak|sk|custom-region|endpoint> <value> [选项]
```

### 参数说明

#### 位置参数

- `<ak|sk|custom-region|endpoint>`：要写入配置文件的 key
  - `ak`：Access Key
  - `sk`：Secret Key（敏感信息）
  - `custom-region`：region（可选值：CN, BOE, SG, BOEI18N, I18N-DEV, Asia-SouthEastBD, I18N-BD）
  - `endpoint`：API base URL（示例：`https://fornax.bytedance.net`）
- `<value>`：对应 key 的值
  - `ak/sk`：直接填明文（写入本地配置文件后请注意文件权限与泄露风险）
  - `endpoint`：推荐写成 `https://...` 的完整 URL；无需尾部 `/`

#### 可写入的 key

- `ak`：Access Key
- `sk`：Secret Key（敏感信息）
- `custom-region`：region（可选值：CN, BOE, SG, BOEI18N, I18N-DEV, Asia-SouthEastBD, I18N-BD）
- `endpoint`：API base URL（示例：`https://fornax.bytedance.net`）

#### 可选参数

- `--local`：写入当前目录的 `./.fornax-cli/config.yaml`，而非全局配置文件
- `-h, --help`：显示本子命令帮助信息并退出

### 示例

```bash
fornax-cli config set ak <AK>
fornax-cli config set sk <SK>
fornax-cli config set custom-region CN
fornax-cli config set endpoint https://fornax.bytedance.net

fornax-cli config set ak <AK> --local
fornax-cli config set sk <SK> --local
```

## config show

展示当前配置值及其来源（环境变量 / 本地配置文件 / 全局配置文件 / 默认值）。

Secret key（`sk`）会在输出中自动打码。

### 用法

```bash
fornax-cli config show [选项]
```

### 示例

```bash
fornax-cli config show

FORNAX_AK=<AK> FORNAX_SK=<SK> fornax-cli config show
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出

### 输出说明

- 会展示全局配置文件和本地配置文件的路径
- 会展示当前 `ak/sk/custom-region/endpoint` 以及每个值的来源（environment / local config file / global config file / default）
- `sk` 会在输出中自动打码

## config select-workspace

查询当前用户有权限的工作空间列表，支持两种模式：

### 交互模式（默认，`--format pretty`）

在终端中展示空间列表，用户通过 ↑/↓ 方向键选择，回车确认后自动将 `workspace-id` 保存到配置文件。

```bash
fornax-cli config select-workspace
fornax-cli config select-workspace --local
```

### 机器模式（`--format json` 或 `--format raw`）

输出 JSON 格式的空间列表到 stdout，不进行交互，供 Agent 或脚本解析。选择后需手动设置：

```bash
fornax-cli config select-workspace --format json
fornax-cli config set workspace-id <ID>
```

输出示例：

```json
[
  {"space_id": 123, "name": "my-space", "description": "My workspace"},
  {"space_id": 456, "name": "team-space", "description": "Team workspace"}
]
```

### 参数说明

- `--local`：将选中的 workspace-id 保存到当前目录的 `./.fornax-cli/config.yaml`，而非全局配置文件
- `--format`：`pretty`（默认，交互模式）、`json`（缩进 JSON）、`raw`（紧凑 JSON）
- `-h, --help`：显示帮助信息

### 前置条件

需要先通过 `fornax-cli auth login` 完成 SSO 登录，或配置有效的 AK/SK。

### 登录后自动触发

`fornax-cli auth login` 成功后，如果在交互式终端中，会自动触发工作空间选择流程。
