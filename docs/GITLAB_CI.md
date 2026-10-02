# Agent GitLab CI 部署

推送 `main` 自动执行 `test → build → deploy_sit`，`deploy_prod` 必须手动点击。
测试与生产使用同一流水线构建的镜像；目标分别为 `DEVIP`、`PRODIP`。
与 MCP 相同，Runner 需要能执行 Docker 构建/推送，目标服务器的 SSH 用户需要能直接执行 Docker。
所有部署步骤位于根目录 `.gitlab-ci.yml`，无需部署脚本目录。

## GitLab 变量

在 **TongjiStudentAgent 项目**的 Settings → CI/CD → Variables 中配置，或由父组继承。
本地 `.env` 不会上传、打包进镜像或自动同步到 GitLab；请把其中实际使用的字段逐项录入。
数据库、Redis 和 MCP 地址使用 `_SIT` / `_PROD` 后缀；其他应用字段与 `.env` 同名，在测试和生产中共用。所有变量的 Environment scope 保持默认 `*`。

变量值直接填写内容，不带 `.env` 外层引号和行尾注释。关闭变量引用展开（Expand variable reference），避免密钥中的 `$` 被替换。可掩码的密钥启用 Masked；若启用 Protected，确保 `main` 是受保护分支。

### 部署连接与存储

| GitLab 变量 | 用途 |
| --- | --- |
| `DEVIP` / `PRODIP` | 测试/生产 SSH 主机地址 |
| `USER` / `PASSWORD` / `PORT` | SSH 用户、密码、端口；`PORT` 不是应用端口 |
| `POSTGRES_DSN_SIT` | 测试 PostgreSQL 连接串，部署时映射为容器内 `POSTGRES_DSN` |
| `POSTGRES_DSN_PROD` | 生产 PostgreSQL 连接串，部署时映射为容器内 `POSTGRES_DSN` |
| `REDIS_URL_SIT` | 测试 Redis 连接串，部署时映射为容器内 `REDIS_URL` |
| `REDIS_URL_PROD` | 生产 Redis 连接串，部署时映射为容器内 `REDIS_URL` |
| `MCP_SERVER_URL_SIT` | 测试 MCP 地址，部署时映射为容器内 `MCP_SERVER_URL` |
| `MCP_SERVER_URL_PROD` | 生产 MCP 地址，部署时映射为容器内 `MCP_SERVER_URL` |

这组变量可使用默认 Environment scope `*`。生产连接请填写生产实际地址，不能自动沿用测试库。
不要另外创建无后缀的 `POSTGRES_DSN` / `REDIS_URL` / `MCP_SERVER_URL`；流水线只使用当前环境的后缀版本，没有跨环境回退。
`CI_REGISTRY`、`CI_REGISTRY_IMAGE`、`CI_REGISTRY_USER`、`CI_REGISTRY_PASSWORD` 由 GitLab Container Registry 提供，无需手工复制。

### 应用字段完整清单

以下覆盖当前 `.env` 的全部字段，同时支持 `.env.example` 中的可选字段。
环境差异只通过上述六个后缀变量区分，不使用 `sit` / `prod` 变量作用域。CORS 白名单和 OAuth 回调地址使用共用的 `CORS_ALLOW_ORIGINS` 和 `TONGJI_OPEN_PLATFORM_REDIRECT_URI`。
如果之前配置过带 `sit` / `prod` 作用域的变量，请迁移为这里的名称并将作用域改为 `*`。流水线保留的 `environment.name` 用于标识部署环境并选择后缀。

| 字段 | 配置说明 |
| --- | --- |
| `APP_PORT` | 必填，建议 `8080`；容器内外使用同一端口 |
| `CORS_ALLOW_ORIGINS` | JSON 数组，例如 `["https://app.tongji.edu.cn","http://<FE-IP>:<FE-PORT>"]`；测试/生产共用，缺省不允许跨域 |
| `MODEL_PROVIDER` | `ark` 或 `openrouter`，缺省 `ark` |
| `LITE_MODEL` | 必填，当前供应商的模型标识 |
| `PRO_MODEL` / `MAX_MODEL` | 可留空，配置后启用对应档位 |
| `OPENROUTER_API_KEY` / `OPENROUTER_BASE_URL` | OpenRouter 凭据与地址，选用该供应商时配置 |
| `ARK_API_KEY` / `ARK_BASE_URL_CN` | Ark 凭据与地址，选用该供应商时配置 |
| `VOLC_API_KEY` | 启用 Ark 知识库时必填 |
| `ARK_KNOWLEDGE_ENABLED` | 知识库开关，按当前 `.env` 配置 |
| `ARK_KNOWLEDGE_PROJECT` | 知识库项目 |
| `ARK_KNOWLEDGE_RESOURCE_ID` | 知识库资源 ID，启用时与 COLLECTION 至少配置一项 |
| `ARK_KNOWLEDGE_LIMIT` / `ARK_KNOWLEDGE_DOMAIN` | 检索条数与知识库服务域名 |
| `MCP_SERVER_URL_SIT` / `MCP_SERVER_URL_PROD` | 必填，`MCP_SERVER_URL_SIT` 填 `http://<DEVIP实际值>:3100/mcp`，`MCP_SERVER_URL_PROD` 填 `http://<PRODIP实际值>:3100/mcp`；不要填写容器的 localhost |
| `TAVILY_ENABLED` / `TAVILY_API_KEY` | 搜索开关与凭据，启用时需凭据 |
| `SANDBOX_ENABLED` | 当前按 `.env` 设置，通常 `false` |
| `COZELOOP_ENABLED` | CozeLoop 开关 |
| `COZELOOP_WORKSPACE_ID` | CozeLoop 工作区，启用时必填 |
| `COZELOOP_JWT_OAUTH_CLIENT_ID` | CozeLoop OAuth 客户端，启用时必填 |
| `COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID` | CozeLoop 公钥 ID，启用时必填 |
| `COZELOOP_JWT_OAUTH_PRIVATE_KEY` | 启用时必填，完整多行 PEM；支持普通 Variable 或 File 类型，保留真实换行，不添加外层引号 |
| `TONGJI_LOGIN_CLIENT_ID` / `TONGJI_LOGIN_CLIENT_SECRET` | 登录应用凭据，必填 |
| `TONGJI_MCP_CLIENT_ID` / `TONGJI_MCP_CLIENT_SECRET` | MCP OpenAPI 应用凭据，必填 |
| `TONGJI_OPEN_PLATFORM_REDIRECT_URI` | 必填，须与开放平台登记的回调地址一致；测试/生产共用 |
| `TONGJI_OPEN_PLATFORM_STATE_SECRET` | 必填，OAuth state 签名密钥 |
| `TONGJI_OPEN_PLATFORM_AUTHORIZATION_ENDPOINT` | 授权地址，按 `.env` 配置或使用服务默认值 |
| `TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT` | Token 地址，按 `.env` 配置或使用服务默认值 |
| `TONGJI_OPEN_PLATFORM_API_BASE_URL` | OpenAPI 地址，按 `.env` 配置或使用服务默认值 |
| `TONGJI_OPEN_PLATFORM_TIMEOUT_SECONDS` | 开放平台请求超时秒数 |
| `ARK_KNOWLEDGE_COLLECTION` | 可选，知识库集合名 |
| `SESSION_ANONYMOUS_TTL` | 可选，匿名会话有效期，例如 `24h` |
| `SESSION_ANONYMOUS_MAX_MESSAGES` / `SESSION_HISTORY_MAX_MESSAGES` | 可选，会话消息上限 |
| `ARK_BASE_URL` | 可选，代码支持的 Ark 地址覆盖项 |
| `FUNC_TIMEOUT` | 可选，优雅退出等待秒数；调整时同步考虑流水线的 Docker 停止等待时间 |

PEM 在 CI 中进行 shell 安全引用，经 SSH 临时文件传送后导出；Docker 命令只传变量名，保留换行、引号和 `$` 原文。运行配置不作为 artifact，也不写入镜像。临时文件在部署结束清理。不要开启 `CI_DEBUG_TRACE` 或在脚本中打印运行配置。

## 运行与验证

- 应用暴露 `http://<目标IP>:<APP_PORT>`；默认建议 `8080:8080`，确认主机端口可用且访问链路放行。
- 当前测试/生产共用 `APP_PORT`，因此 `DEVIP` 与 `PRODIP` 应指向不同服务器，避免端口冲突。
- Agent 启动时连接 PostgreSQL、Redis、MCP；部署前先确保对应环境的 MCP 已启动，并能被 Agent 容器访问。
- 服务使用外部 PostgreSQL / Redis，不在部署时创建或清空数据库，也不迁移 Railway 数据；数据库表结构由现有服务启动逻辑维护。
- Docker 健康检查请求 `/v1/ping`。流水线等待新容器健康，失败时恢复原容器；成功后删除旧容器。容器回退不会撤销数据库结构变化。
- 保留非 root 用户与 Chromium，分配 `256m` 共享内存，配置容器重启和日志轮转。
- GitLab 生产按钮首次执行前，确认 `POSTGRES_DSN_PROD`、`REDIS_URL_PROD` 以及 `MCP_SERVER_URL_PROD` 已配置，并确认共用的回调地址和 CORS 白名单适用于生产。

## Go 依赖下载

测试和 Docker 构建统一使用 `GOPROXY=https://goproxy.cn,direct`，保留 Go 默认 checksum 校验。无需新增 GitLab 变量；需要内部代理时可覆盖 CI 变量 `GOPROXY`。代理只用于构建，不注入运行容器。

`proxy.golang.org ... i/o timeout` 表示依赖下载失败，尚未开始部署。请推送修复后的新提交；重试旧流水线仍会使用旧配置。

## 系统依赖下载

Docker runtime 安装 Chromium、中文字体等依赖时使用清华 Debian / Debian Security 镜像。基础镜像尚未安装 CA 证书，使用 HTTP 引导，保留 Debian 签名和哈希校验。安全更新镜像可能有同步延迟。APT 网络超时 30 秒，失败重试 2 次；更新及安装整体限制为 900 秒。无需新增 GitLab 变量。

修改只影响新提交的流水线，不会改变正在运行的构建。可取消旧 build 后推送修复。
