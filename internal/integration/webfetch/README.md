# Webfetch：公开网页正文与候选链接提取

`webfetch` 安全地读取一个公开页面，提取服务器端与浏览器端正文，合并内容，并返回经过校验的候选链接。默认策略以内容完整性优先：支持的 HTML 即使已有静态正文，也会尝试 Chromium 渲染。

本包不依赖 Tavily，不负责搜索问题规划、模型提示词、工具授权或最终回答。`system.url_fetch` 在上层调用本包，负责工具参数验证、正文字符数截断、不可信内容包装及 JSON 输出。

## 文件职责

| 文件 | 职责 | 主要入口 |
| --- | --- | --- |
| `types.go` | 定义提取输入、输出、浏览器契约和稳定错误类型。 | `ExtractInput`、`ExtractResponse`、`RenderedPage`、`Browser`、`Error`、`Status` |
| `client.go` | 装配默认依赖、提供浏览器启动预检，并协调提取、合并、链接校验、取消、降级和日志。 | `NewFromEnv`、`VerifyChromium`、`NewClient`、`Client.Extract`、`Client.render` |
| `decode.go` | 检查媒体类型，将支持的字符集转换为有界 UTF-8 内容。 | `decodePage` |
| `content.go` | 从 HTML、hydration JSON 和 `noscript` 提取候选正文，排列优先级并合并。 | `extractCandidates`、`mergeCandidates`、`mergeText` |
| `links.go` | 提取锚点、补全相对链接、解析 DuckDuckGo 跳转，校验并限制返回链接。 | `Link`、`extractLinks`、`normalizeLink`、`validateLinks` |
| `network.go` | 统一 URL/DNS 校验、公网 IP 分类、连接时校验和 HTTP 重定向检查。 | `publicNetwork`、`validateURL`、`resolve`、`dialContext`、`client` |
| `proxy.go` | 提供 Chromium 的本地出口代理，转发 HTTP 请求与 HTTPS CONNECT 隧道。 | `browserProxy`、`startProxy`、`ServeHTTP`、`connect` |
| `chromium.go` | 实现浏览器启动、请求拦截、JS 执行、正文与链接读取，以及进程和代理清理。 | `ChromiumConfig`、`NewChromiumBrowser`、`Preflight`、`Render`、`allocator` |

阅读源码时，建议从 `types.go` 和 `client.go` 开始，再分别沿内容提取链路和网络访问链路展开。

## 分层与调用关系

```text
system.url_fetch                         工具输入、授权、结果封装
  └─ Client.Extract                      提取流程协调
      ├─ publicNetwork → HTTP GET        安全读取原始响应
      ├─ decodePage                      类型检查与字符集转换
      ├─ extractCandidates               静态正文候选
      ├─ extractLinks                    静态链接候选
      ├─ Browser.Render                  动态正文与链接
      │   └─ Chromium
      │       ├─ handlePaused            请求发出前校验
      │       └─ browserProxy
      │           └─ publicNetwork       实际连接时校验并绑定公网 IP
      ├─ mergeText                       合并正文表示
      └─ validateLinks                   校验、去重并裁剪候选链接
```

两条网络路径共享 `publicNetwork`；共同的 URL 格式及凭据策略来自 `internal/platform/publicurl`。因此底层抓取与上层工具可以使用同一策略，无需让 integration 层依赖 Agent 工具层。

`content.go` 只处理内容，不发起网络请求。`links.go` 中的解析阶段也不联网，但最终链接校验会解析 DNS；它不会抓取链接目标页面。

## 初始化与资源生命周期

`NewFromEnv` 装配默认公网 HTTP 客户端和 Chromium 渲染器，本身不启动浏览器进程。`NewClient` 支持注入 HTTP 客户端与 `Browser`，便于离线测试；传入 `nil` 浏览器时，仅进行 HTTP 提取。注入的 HTTP 客户端由调用方负责连接安全，默认构造才会自动装配本包的公网传输层。

应用启动阶段单独调用 `VerifyChromium`。它创建浏览器执行环境并打开 `about:blank`，检查可执行文件及运行环境能否正常启动。预检失败时应用记录 warning 并调用 `DisableBrowser`，服务继续启动，后续请求只执行 HTTP 提取。预检不证明任意网站都能提取成功。

浏览器路径优先读取 `CHROME_BIN`，否则在 PATH 中查找 `chromium`、`chromium-browser`、`google-chrome` 和 `google-chrome-stable`。PATH 未找到时，继续探测 macOS 的 `/Applications`、`~/Applications` 下的 Google Chrome/Chromium，以及 Linux 的 `/usr/bin`、`/opt/google/chrome/chrome`、`/snap/bin/chromium`。显式配置无效时直接报错。生产镜像需提供可执行的 Chromium。

每次 `Render` 都建立独立的执行环境与本地代理，结束后关闭浏览器和代理；当前没有常驻浏览器池。代理的 CONNECT 连接额外绑定上下文与截止时间，因为 HTTP server 的关闭不会自动回收已接管的隧道连接。

## 单次提取流程

1. 检查上下文和客户端，默认网络路径校验目标 URL 与 DNS。
2. 执行有界 HTTP GET，检查响应状态并读取响应体。
3. `decodePage` 检查媒体类型、解码字符集，统一为 UTF-8。
4. 纯文本归一化后直接返回，来源为 `http_text`，不启动浏览器。
5. HTML/XHTML 提取所有静态正文候选和链接候选。
6. 配置了浏览器时，顺序执行 Chromium 渲染，将动态正文与静态正文合并，并汇总链接候选。
7. 返回前统一校验链接的 URL、DNS、数量与标题长度。

HTTP 和浏览器阶段当前是顺序执行，各自具有超时；30 秒浏览器超时不等于整个 `Extract` 调用的总超时。调用方可以通过 context 设置总体预算。

## 正文解析与合并

### 类型和编码

支持 `text/html`、`application/xhtml+xml` 与 `text/plain`。缺失 Content-Type 时探测类型；明确不支持的媒体类型、格式错误的声明或未知字符集会被拒绝，不会再通过浏览器兜底。

字符解码依据 BOM、HTTP charset 与 HTML meta 声明进行。正文和链接标题使用同一份解码后的数据。PDF、图片、压缩包及独立 JSON API 响应不属于当前提取范围。

### 静态候选

| 来源标识 | 提取内容 | 合并优先级 |
| --- | --- | --- |
| `http_main` | `article`、`main`、`role=main` 节点正文。 | 1 |
| `http_hydration_json` | `application/json` 脚本中符合条件的富 HTML 字符串。 | 2 |
| `http_noscript` | 无脚本备用 HTML。 | 3 |
| `http_body` | 普通 body 文本。 | 4 |

同一优先级内，较长的候选排在前面。Hydration 提取递归查找至少 80 字节且含正文类 HTML 标签的字符串，并按键排序遍历 JSON 对象以保持结果确定性；它不是任意 JavaScript 状态求值器。

文本提取忽略脚本、样式及其他非正文节点，并以换行表达段落、列表及表格单元格边界。它不保留页面视觉布局或原始 Markdown 格式。

### 合并语义

`normalizeText` 归一化实体与空白，保留非空行的顺序和重复内容。

`mergeText` 处理不同表示之间的完整片段包含关系，以及按完整文本行匹配的首尾重叠。`suffixPrefixOverlap` 使用前缀表计算重叠长度；孤立且少于 80 个字符的重复行不作为段落重叠依据。

这里不会全局删除相同行。例如两个适用群体各自拥有相同截止日期时，这两处事实都需要保留。若后一个表示完整包含已有正文，合并结果可以替换成这个更完整的表示，因此候选优先级不保证最终文本始终以某个 main 节点开头。

## 浏览器渲染与网络保护

`Render` 导航到目标页，等待 body 出现，额外等待 750ms 后读取 `document.body.innerText`、最终 URL 和锚点链接。它不自动滚动、点击“加载更多”、登录或处理验证码。

浏览器访问包含两层检查：

- `handlePaused` 检查主文档及子资源请求，放行符合规则的请求；`data:`、`blob:`、`about:` 作为允许的资源协议处理。
- `browserProxy` 处理 HTTP 转发和 HTTPS CONNECT，使用 `publicNetwork.dialContext` 重新解析目标并直接连接已验证的公网 IP，避免仅凭先前的 DNS 检查放行连接。

Chromium 配置强制使用代理、取消 loopback 自动绕行、限制本地目标域名解析，并禁用 QUIC 和非代理 WebRTC UDP。HTTPS 隧道不解密 TLS 内容；具体页面 URL 的查询参数检查由请求拦截承担，隧道层校验主机、端口及实际连接目标。

`publicNetwork` 拒绝非公网地址、私网、回环、链路本地、CGNAT 和指定文档 IPv6 地址；DNS 结果中只要存在不允许的地址，就拒绝整个结果。HTTP 传输层不使用环境代理，每次新连接重新校验目标。

## 候选链接与返回结果

`extractLinks` 读取 HTML 锚点，结合页面 URL 与首个有效处理的 base 地址解析相对引用，跳过空链接与纯片段链接。浏览器阶段额外提供动态 DOM 中的锚点。

`normalizeLink` 校验公开 URL，并将 DuckDuckGo `/l/` 链接的 `uddg` 参数还原为目标地址，再次执行 URL 校验。最终 `validateLinks` 对合并后的候选进行去重及公网 DNS 检查；主机解析结果只在当前这批校验中复用，后续真正访问仍会重新校验。

`ExtractResponse` 的字段职责：

| 字段 | 含义 |
| --- | --- |
| `URL` | 本次返回结果对应的页面地址。 |
| `Content` | 合并后的归一化正文；尚未执行上层工具的 `max_chars` 裁剪。 |
| `Source` | 参与提取的来源标识，多个标识以 `+` 连接；不表示每个来源都有独有内容。 |
| `Links` | 已校验的候选来源链接，不代表目标正文已读取。 |
| `LinksTruncated` | 候选收集或校验因数量、时间等边界未能完整处理。 |

正文与候选链接分开返回，Agent 需要通过后续 fetch 才能核验候选页面。

## 限制与失败处理

| 限制 | 当前值及范围 |
| --- | --- |
| HTTP 请求超时 | 15 秒。 |
| 原始及解码后页面大小 | 各最多 4 MiB；不等同于合并后正文上限。 |
| HTTP 重定向 | `maxRedirects=5`，检查时 `len(via) >= 5` 拒绝继续跳转。 |
| 默认浏览器超时 | 30 秒。 |
| body 就绪后的额外等待 | 750ms。 |
| 链接候选 | 静态与浏览器各最多收集 200 个，最终校验最多处理合并列表的前 200 个。 |
| 返回链接 | 最多 50 个，每个标题最多 300 个 Unicode 字符。 |
| 链接校验预算 | 一批最多 2 秒，并受调用方上下文约束。 |

HTTP 传输失败或非 2xx 时，有浏览器则尝试浏览器路径；传输错误中的明确安全拒绝不会触发该降级。响应读取失败、超限或解码拒绝直接返回错误。

HTML 静态正文已经可用时，浏览器失败仍可返回静态结果；若调用方 context 已取消，则传递取消错误，不伪装为成功。当前结果没有单独的浏览器降级原因字段，调用方不能仅凭 `Source` 缺少 `browser` 就判断失败原因。

`Error` 使用 `url_not_allowed`、`timeout`、`fetch_failed`、`web_unavailable` 等稳定状态。`Status` 对其他错误默认映射为 `web_unavailable`；调用方需要单独保留 context 取消语义。

提取日志记录来源、稳定状态和耗时，不记录正文或完整 URL。当前没有自动翻页、无限滚动、图片理解或真实浏览器池，因此成功提取不等于覆盖页面所有交互状态。

## 测试范围

| 文件 | 覆盖行为 |
| --- | --- |
| `client_test.go` | 默认客户端装配（不启动浏览器进程）、静态与动态正文、链接输出、取消、失败降级、响应大小与资源关闭。 |
| `content_test.go` | 重复事实保留、重叠正文合并、字符集与二进制响应的集成行为。 |
| `decode_test.go` | UTF-8、GBK、BOM、媒体类型拒绝、解码大小与链接标题编码。 |
| `links_test.go` | 相对链接、base、DuckDuckGo 跳转、DNS 校验、去重与输出限制。 |
| `network_test.go` | URL/公网校验、混合 DNS 拒绝、DNS 重绑定、IP 绑定及重定向限制。 |
| `proxy_test.go` | HTTP 转发、CONNECT 隧道、头部过滤、公网目标校验及隧道生命周期。 |
| `chromium_test.go` | 浏览器请求准入和无效可执行文件路径，不覆盖真实页面的 JS 渲染效果。 |

从仓库根目录运行：

```bash
go test ./internal/integration/webfetch
```

测试使用固定响应、模拟 DNS 与网络管道，不依赖真实外部站点；代理隧道测试需要监听临时本地端口。生产 Chromium 的真实可启动性由启动预检验证，实际页面渲染质量仍需另行验收。
