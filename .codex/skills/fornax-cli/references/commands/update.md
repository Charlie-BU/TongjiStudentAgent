# fornax-cli update

`update` 用于更新 CLI 二进制，并按版本 stamp 同步平台托管的 `fornax-cli` skill。

## update

更新 CLI 到最新版本或指定版本。CLI 会先比较当前二进制版本与目标版本，版本已匹配时跳过二进制覆盖安装；CLI 更新完成后按 `~/.fornax-cli/skills.stamp` 同步 `fornax-cli` skill。

### 用法

```bash
fornax-cli update [选项]
```

### 示例

```bash
fornax-cli update
fornax-cli update --version v0.1.0
fornax-cli update --force
```

### 参数说明

- `-h, --help`：显示帮助信息并退出
- `--version <version>`：指定要安装的 CLI 版本；不传时更新到 latest
- `--force`：强制执行 skill 同步

## update skill

单独同步平台托管的 `fornax-cli` skill。

### 用法

```bash
fornax-cli update skill [选项]
```

### 安装方式

命令会调用：

```bash
npm_config_registry="https://bnpm.byted.org" npx -y skills@latest add skills.byted.org/stone/fornax --skill fornax-cli -g -y
```

同步成功后写入 `~/.fornax-cli/skills.stamp`。stamp 与当前 CLI 版本匹配时跳过同步；传入 `--force` 时强制执行。`npx` 缺失或 `skills` CLI 执行失败时，命令会打印可手动执行的命令。

### 示例

```bash
# 安装平台托管的 fornax-cli skill
fornax-cli update skill

# 强制同步平台托管的 fornax-cli skill
fornax-cli update skill --force
```

### 参数说明

- `-h, --help`：显示本子命令帮助信息并退出
- `--force`：强制执行 skill 同步
