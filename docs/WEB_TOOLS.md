# 公开网页工具 (For Human)

`system.web_search` 和 `system.url_fetch` 是 Agent 面向公开互联网的两项受控系统工具。二者经常连续使用，却是两条独立链路：

| 工具 | 目标 | 当前实现 | 是否依赖 Tavily |
| --- | --- | --- | --- |
| `system.web_search` | 找到候选公开来源和有界摘要 | Tavily Search REST API | 是 |
| `system.url_fetch` | 读取单个公开 URL 的可核验正文 | 本地自适应 HTTP 提取器，必要时本机 Chromium | 否 |

校园 MCP Server、同济开放平台 Access Token、Cookie 和会话历史均不参与这两条链路。网页工具只能处理公开资料；成绩、课表、校园卡、借阅记录等用户实时数据必须走相应的 Tongji MCP 工具。

## 设计目标与边界

网页搜索解决“去哪里找”，网页正文提取解决“页面实际写了什么”。搜索摘要不是已读正文，模型不能将摘要伪装为已核验来源；正文提取成功也不代表该页面适用于当前用户、当前年份或所有学院。回答仍需结合来源的时效、发布主体和适用范围。

工具刻意不提供以下能力：

- 登录、传入 Cookie、OAuth code、Token 或任何校园凭据；
- 绕过验证码、付费墙、访问控制或反爬策略；
- 多页爬取、站点镜像、`fetch_id`、缓存或页面内分页；
- 对网页内任何文本、提示词、脚本或链接的执行授权。

所有上游标题、摘要和正文都会包进 `<untrusted_web_data>`。这表示它们只能用作事实参考，绝不能被解释为系统指令、工具调用或授权。

## Agent 调度上下文

系统提示词将公开知识查询约束为“知识库 + 网页”联合核验：

1. 对校规、校历、校园服务、学院通知等公开知识，模型同时发起 `system.search_knowledge` 与 `system.web_search`；两者没有前后依赖，可以并行。
2. 收到搜索结果后，模型只能使用其中真实返回、与问题相关的 URL 调用 `system.url_fetch`。搜索到正文存在数据依赖；多个独立 URL 可以并行提取。
3. 知识库有有效资料时，它是第一可信来源，网页用于补充与交叉核验；知识库无有效资料时，模型以已核验的公开网页为主要依据，并明确说明依据切换。
4. 任何链路无有效来源时，模型必须说明不确定性，不能编造调用、正文或引用。

用户直接给出公开 URL 并要求阅读、总结或核验时，可以直接调用 `system.url_fetch`；不要求先搜索。用户问题涉及“最新”或需要找到来源时，通常先搜索再提取更合适。

## 启动、注册与配置

聊天服务在启动时分别创建两个客户端：

```text
chat.NewFromEnv
 ├─ tavily.NewFromEnv()       ──可选──> system.web_search
 └─ webfetch.NewFromEnv()     ──默认──> system.url_fetch
```

`systemtools.Tools` 只有在工具 allowlist 允许时才向模型注册对应工具。两项工具的可用性相互独立：关闭 Tavily 会移除搜索工具，但不会移除正文提取工具。

### 搜索配置

| 环境变量 | 含义 |
| --- | --- |
| `TAVILY_ENABLED` | 可选布尔值，缺省 `false`；仅控制 `system.web_search`。 |
| `TAVILY_API_KEY` | 当 `TAVILY_ENABLED=true` 时必填；只用于 Tavily Bearer 鉴权。 |

Tavily client 的总超时固定为 30 秒。启用但未配置 Key、或开关不是布尔值时，服务启动失败，避免模型看见一个永远不可用的搜索工具。

### 正文提取配置

`system.url_fetch` 默认启用，没有 Tavily 开关或 API Key，也没有额外的 URL Fetch 环境变量。HTTP 快路径超时固定为 15 秒，Chromium 渲染超时固定为 30 秒。生产镜像必须安装兼容的 Chrome/Chromium；应用优先使用 `CHROME_BIN`，否则先在 PATH 中查找 `chromium`、`chromium-browser`、`google-chrome`、`google-chrome-stable`，再探测常见安装路径。macOS 支持 `/Applications` 和 `~/Applications` 下的 Google Chrome、Chromium；Linux 支持 `/usr/bin`、`/opt/google/chrome/chrome` 和 `/snap/bin/chromium`。显式配置的 `CHROME_BIN` 无效时直接报错，不自动回退。

部署时，`webfetch.VerifyChromium` 会在 HTTP listener 启动前启动浏览器并打开 `about:blank`。因此缺少浏览器、动态库或可用 sandbox 时启动直接失败，Railway 的 `GET /v1/ping` 健康检查不会成功；这比等到某次 URL 需要 JavaScript 回退时再报错更早、更可观测。仓库根目录的 `Dockerfile` 固定使用 Debian Bookworm 和 `chromium`，`railway.json` 强制使用该镜像并指向 `/v1/ping`。Railway 服务的 Root Directory 应设为 `TongjiStudentAgent`。

`reason` 仅用于工具调用说明，不会发送给 Tavily、目标网站或浏览器。两个工具都不会转发用户请求头、Access Token、Cookie、会话历史或学生信息。

## `system.web_search`

### 输入

| 参数 | 约束 | 说明 |
| --- | --- | --- |
| `query` | 必填，1～500 字符 | 公开搜索问题；不得包含凭据或私有数据。 |
| `reason` | 必填，1～120 字符 | 面向用户的简短调用原因。 |
| `max_results` | 1～10，默认 5 | 返回的最多来源数。 |
| `start_date` / `end_date` | 可选 `YYYY-MM-DD` | Tavily 的发布/更新时间过滤；不承诺精确时区边界。 |
| `include_domains` / `exclude_domains` | 各最多 10 个 | 仅接受域名，不接受协议、路径或任意 URL。 |

参数 JSON 必须是一个不超过 16 KiB 的对象，未知字段会被拒绝。日期必须有效，且起始日期不得晚于结束日期。

### Tavily 调用与结果

当前适配器向 `https://api.tavily.com/search` 发起 POST，请求固定为：

- `topic=general`；
- `search_depth=advanced`；
- `auto_parameters=false`、`include_answer=false`、`include_raw_content=false`、`include_images=false`；
- 仅发送经过校验的搜索参数和 Tavily Bearer Key。

结果不是原样透传。服务会丢弃空摘要、无效 URL 和重复 URL，最多保留 `max_results` 条来源；标题最多 300 个 Unicode 字符，摘要最多 1500 个 Unicode 字符。成功响应为：

```json
{
  "status": "ok",
  "query": "...",
  "sources": [
    {
      "title": "<untrusted_web_data ...>",
      "url": "https://...",
      "snippet": "<untrusted_web_data ...>",
      "truncated": false
    }
  ],
  "message": "已检索到公开网页参考资料，请按来源核验。"
}
```

搜索结果只提供候选来源和摘要。模型应从 `sources[].url` 选择相关页面交给 `system.url_fetch`，再将最终回答附上实际来源链接。

### 搜索失败语义

Tavily 的原始错误正文不会进入模型上下文、SSE 或普通日志。稳定状态如下：

| 状态 | 含义 |
| --- | --- |
| `invalid_arguments` | 输入校验失败，或 Tavily 返回 HTTP 400。 |
| `rate_limited` | Tavily 返回 HTTP 429。 |
| `quota_exceeded` | Tavily 返回 HTTP 432 或 433。 |
| `no_results` | HTTP 调用成功，但没有可用的非空来源。 |
| `timeout` | 请求或读取响应超时。 |
| `web_unavailable` | 供应商、网络、鉴权、协议或响应体异常。 |
| `tool_not_allowed` | 工具未启用或未在 allowlist 中。 |

本轮 context 被取消时，调用返回 cancellation error，而不是伪装成可恢复的网页结果。

## `system.url_fetch`

### 输入与输出

| 参数 | 约束 | 说明 |
| --- | --- | --- |
| `url` | 必填，最多 2048 字节 | 仅允许公开 HTTP/HTTPS URL；不允许认证信息。 |
| `reason` | 必填，1～120 字符 | 面向用户的调用原因。 |
| `query` | 可选，最多 500 字符 | 为兼容旧调用保留；当前不做相关片段筛选。 |
| `max_chars` | 1000～16000，默认 8000 | 对最终纯文本按 Unicode 字符截断。 |

成功时返回：

```json
{
  "status": "ok",
  "url": "https://final-public-url.example/...",
  "content": "<untrusted_web_data kind=\"content\">...</untrusted_web_data>",
  "content_mode": "full",
  "fetch_mode": "http_main",
  "truncated": false,
  "message": "已提取并汇总公开网页的可用正文表示。"
}
```

正文是归一化纯文本，不是 Markdown，也不保证保留原页面视觉布局、图片、表格样式或交互状态。链接以独立 `links` 数组输出。`fetch_mode` 以 `+` 连接本次参与合并的路径，例如 `http_main+http_hydration_json+browser`；它说明内容来源，不表示页面的视觉布局被保留。

### 结构化候选链接

成功响应额外包含 `links` 和 `links_truncated`。`links` 返回最多 50 条 `{url, title}`，标题最多 300 个 Unicode 字符并经不可信数据包装。HTTP 从锚点 `href` 和首个 `<base href>` 解析绝对地址，Chromium 补充动态锚点。相同 URL 去重并移除 fragment；DuckDuckGo `/l/?uddg=...` 跳转会解码为目标地址。

返回前共享 URL 策略拒绝非 HTTP/HTTPS、私网 IP、非标准端口、userinfo 和敏感认证参数，再验证 DNS 的所有地址均为公网。每份结果最多检查 200 个候选，DNS 校验总预算 2 秒，同一主机在本次结果内缓存。超过预算或数量限制时，`links_truncated=true`；它与正文截断标记独立。链接校验不抓取目标正文，下一次 fetch 会重新校验并绑定连接 IP。

### 媒体类型与字符集

HTTP 成功响应仅支持 `text/html`、`application/xhtml+xml` 和 `text/plain`。缺少 Content-Type 时先嗅探类型；显式不支持的类型（如 PDF、图片、ZIP、JSON）、损坏的类型声明或未知 charset 返回 `fetch_failed`，不会作为 HTML 或浏览器备用正文使用。

提取正文和链接前，按 BOM、HTTP charset 和 HTML meta 声明解码为 UTF-8，支持 GBK 等编码。原始响应和解码后的文本均受 4 MiB 上限约束。空纯文本也返回 `fetch_failed`。

### 自适应提取链路

正文提取器不再调用 Tavily Extract。它一次读取单个 URL，并把可获得的服务器端与浏览器端表示合并为一个去重后的正文；完整性优先于单次调用的成本：

```text
已校验的公开 URL
  │
  ├─ HTTP GET（15 秒、最多 4 MiB、最多 5 次重定向）
  │   ├─ text/plain                         → http_text
  │   ├─ <main> / <article> / [role=main]   → http_main
  │   ├─ 普通 <body>                        → http_body
  │   ├─ <script type="application/json">  → http_hydration_json
  │   └─ <noscript>                         → http_noscript
  │
  ├─ HTML 成功响应
  │   └─ Chromium 渲染（30 秒）              → browser
  │
  └─ 合并结构化 HTTP 候选与 browser 可见文本
      └─ 保留去重后的最完整正文
```

HTML 提取会忽略 `script`、`style`、`template`、`svg`、`canvas` 和 `iframe` 等非正文节点；正文块和列表会转换为分行纯文本，再做实体解码与空白归一化。不同表示按完整行边界判断包含关系，并合并有足够上下文的末尾/开头重叠段；同一表示内的重复段落、标题和表格值全部保留，孤立的短重复值不作为重叠证据。这样不会把 main、body 和浏览器中的相同正文反复计入输出字数。解析器优先合并语义正文、hydration JSON、`noscript` 和 `body`，随后追加浏览器可见文本；浏览器启动失败不会丢弃已有 HTTP 内容。

#### 普通 HTML：`http_main` 与 `http_body`

解析器收集 `<article>`、`<main>` 和 `role=main`，再收集 `<body>`。语义正文的优先级高于宽泛的 `body`，以便在工具输出受 `max_chars` 限制时尽早保留正文；它们都会参与最终合并，而不是以“足够长”为由阻止浏览器渲染。

#### Hydration JSON：`http_hydration_json`

现代 SSR/SPA 通常把初始状态嵌入：

```html
<script type="application/json" id="goose-payload">
  {"props":{"page":{"content":"<h2>参赛流程</h2><ol>...</ol>"}}}
</script>
```

提取器会解析所有 `type="application/json"` 脚本，递归查找长度至少 80 字节且含 `main`、段落、标题、列表、表格、文章或 section 标签的字符串，把这些富文本再次按 HTML 解析。它不是框架专属逻辑，因此可覆盖 Next/Nuxt 或自定义 payload；它也不会把任意脚本源码当成正文。

#### `<noscript>`：`http_noscript`

站点可为禁用 JavaScript 的用户、搜索引擎和爬虫提供备用 HTML：

```html
<noscript><main><h1>标题</h1><ol><li>完整步骤</li></ol></main></noscript>
```

解析器单独读取此片段并按 HTML 提取文字。它不是“必须禁用 JS”的信号，而是一个成本很低、往往更完整的内容载体。

#### Chromium：`browser`

对每个成功取得的 HTML 页面，`chromedp` 都会启动无头 Chromium：导航到目标页、等待 `body` 出现、额外等待 750ms，然后读取 `document.body.innerText`。此路径执行页面 JavaScript，得到的可见文本会与 HTTP 结果合并，而不是只在静态内容为空时才使用。HTTP 非 2xx 或传输失败时，浏览器仍会作为唯一可行路径尝试一次。当前不自动滚动、点击“加载更多”、登录或处理验证码，因此懒加载、无限滚动和受保护页面仍可能不完整或失败。

浏览器结果保留渲染后可见文本，未再执行 Readability 一类的主内容清洗；因此它可能包含导航、侧栏或页脚，但优先保证内容完整性。

## URL 与网络安全

URL 在工具层和网络层都要通过校验。允许规则不是“字符串看起来像 URL”即可：

- 仅 `http` / `https`，必须有 hostname；拒绝 userinfo、`localhost`、非标准端口和已知敏感查询参数；
- 端口仅允许 `80` 或 `443`；
- DNS 解析后的所有地址都必须是公网 global-unicast 地址；拒绝私网、回环、链路本地、CGNAT `100.64.0.0/10` 和文档 IPv6 `2001:db8::/32`；
- 连接时再次解析并直接拨号到已验证 IP，避免仅在初检时解析造成的 DNS rebinding；
- 最多跟随 5 次重定向，每次重定向 URL 都重新校验；
- HTTP 响应最多读取 4 MiB。

Chromium 拦截主文档和子资源请求作初步 URL/DNS 校验；`data:`、`blob:` 和 `about:` 子资源可继续。所有 HTTP/HTTPS 请求还必须经过每次渲染独立的本地 HTTP/CONNECT 代理：代理不采用环境代理配置，在实际拨号时重新解析并绑定已校验的公网 IP，消除初检与浏览器独立解析之间的 DNS rebinding 窗口。HTTPS 隧道只连接该 IP，不解密 TLS。

浏览器配置固定代理并用 `<-loopback>` 关闭隐式本地地址绕过，不配置 DIRECT 后备；禁用直接 DNS、QUIC 和非代理 WebRTC UDP。代理随渲染 context 关闭，已 hijack 的 CONNECT 两端也被取消并回收。容器进程沙箱继续启用。代理行为依据 [Chromium 代理规范](https://chromium.googlesource.com/chromium/src/+/refs/heads/main/net/docs/proxy.md)。

## 可恢复错误与观测

`system.url_fetch` 使用以下稳定状态：

| 状态 | 含义 |
| --- | --- |
| `invalid_arguments` | JSON、文本长度或 `max_chars` 不合法。 |
| `url_not_allowed` | URL、端口、DNS 或重定向目标不满足公开网络规则。 |
| `tool_not_allowed` | 工具未注册或未在 allowlist 中。 |
| `timeout` | HTTP 或 Chromium 操作超过各自的超时。 |
| `fetch_failed` | 已访问页面但无法取得有效正文，或响应体超过限制。 |
| `web_unavailable` | 网络、浏览器启动、协议或其他基础设施异常。 |

Tavily 搜索额外可能返回 `rate_limited`、`quota_exceeded` 和 `no_results`，这些不是本地 URL fetch 的状态。

HTTP/Tavily 日志只记录操作、HTTP 状态（Tavily）、正文来源 `fetch_mode`、稳定状态和耗时；不记录完整 URL、搜索词、正文、上游错误正文或 API Key。SSE、会话历史和模型生成的工具参数仍可能包含模型自己写出的 URL/问题，因此不能视为仅状态通道。

## 测试与验收

离线测试覆盖：

- Tavily 的请求参数、认证隔离、HTTP 状态映射、响应大小、取消和超时；
- URL fetch 的普通 HTML、hydration JSON、`<noscript>`、`text/plain`、浏览器合并、浏览器失败后的静态降级、Unicode 截断、allowlist 与 URL 拒绝；
- 注册条件：Tavily 关闭时仅搜索工具消失，正文提取仍注册；
- 公网 DNS 混合地址拒绝、重绑定拒绝、IP 字面量拨号、重定向拒绝/上限、HTTP/CONNECT 代理、代理头隔离、隧道取消与资源回收；
- 跨表示重叠与最终字数上限、GBK 响应头/meta、UTF-8/BOM、媒体类型拒绝及解码大小边界；
- 重复章节事实与表格值保留、相对链接/base、动态链接合并、DuckDuckGo 跳转、凭据与私网链接拒绝、链接数量/标题/取消边界；
- runtime 中的搜索、正文核验和失败后继续回答。

生产验收应在部署环境调用代表性的静态页和动态页，确认 `fetch_mode` 同时包含已合并的 HTTP 与 `browser` 路径，并且浏览器不可用时静态页面仍可正常返回。浏览器可执行性在服务启动阶段已由 `VerifyChromium` 预检。

## 相关实现与外部文档

- `internal/agentic/systemtools/web_search`：工具输入、结果裁剪和 Tavily 错误归一化。
- `internal/agentic/systemtools/url_fetch`：输入校验、工具结果封装、Unicode 截断和 `fetch_mode` 输出。
- `internal/integration/tavily`：仅搜索供应商适配器。
- `internal/integration/webfetch`：SSRF-safe HTTP、HTML/hydration/`noscript` 解析、Chromium 渲染与正文合并。
- [Tavily Search API](https://docs.tavily.com/documentation/api-reference/endpoint/search)
- [chromedp](https://github.com/chromedp/chromedp)
