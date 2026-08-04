# TongjiStudent Agent 单元测试规范

本文以 `reference/UTSpec.md` 的章节、测试组织方式和质量要求为基线，并结合 TongjiStudent Agent 的 Go/Hertz/Eino、知识库与 MCP 架构完成适配。所有新增测试依赖必须为公开可获取的开源依赖；测试不得访问真实模型、同济开放平台、生产知识库或真实学生数据。

## 0. 测试工具

本项目使用以下三个核心测试工具：

### 0.1 GoConvey - 断言工具

- **用途**：提供 BDD 风格的断言和测试组织。
- **优势**：链式断言、嵌套测试场景、可读性强。
- **安装**：`go get github.com/smartystreets/goconvey`
- **文档**：https://github.com/smartystreets/goconvey
- **说明**：当前已有测试可继续使用 Go 标准库 `testing` 的表驱动风格；新增复杂业务场景按本规约使用 GoConvey 组织。

### 0.2 gomonkey - 运行时 Mock 工具

- **用途**：在运行时 mock 函数、方法和全局变量。
- **优势**：无需修改源代码、支持私有方法 mock、适用于无法注入的依赖。
- **安装**：`go get github.com/agiledragon/gomonkey/v2`
- **文档**：https://github.com/agiledragon/gomonkey
- **适用场景**：
  - Mock 第三方库函数。
  - Mock 私有方法。
  - Mock 全局变量或包级默认服务。
  - Mock 暂未抽象为接口的依赖。

### 0.3 gomock - 接口 Mock 工具

- **用途**：基于接口的 mock，适用于 HTTP Client、MCP Client、知识库、Session Store 和校园业务客户端等依赖。
- **优势**：类型安全、支持期望验证、IDE 友好。
- **安装**：

  ```bash
  go install github.com/golang/mock/mockgen@latest
  go get github.com/golang/mock/gomock
  ```

- **适用场景**：
  - Mock MCP/OpenAPI/知识库等外部服务接口。
  - Mock 数据库、Redis、Session 或 Checkpoint 接口。
  - Mock 任何业务拥有的稳定接口。
  - 验证方法调用次数、参数和调用顺序。

### 0.4 工具选择指南

| 场景 | 推荐工具 | 原因 |
|------|---------|------|
| 断言验证 | GoConvey | 提供丰富的断言 API，可读性强 |
| Mock 第三方函数 | gomonkey | 可以 mock 无接口的函数，无需修改生产代码 |
| Mock 私有方法 | gomonkey | 支持私有方法 mock |
| Mock 全局变量 | gomonkey | 支持全局变量和包级变量 mock |
| Mock MCP / OpenAPI / Store 接口 | gomock | 类型安全，支持期望验证 |
| Mock 数据库接口 | gomock 或 sqlmock | 通过接口验证调用，或对 SQL 做确定性断言 |
| Mock 不支持接口的依赖 | gomonkey | 无需接口即可隔离依赖 |
| 需要验证调用次数/参数 | gomock | 内置期望验证机制 |
| HTTP 协议适配 | httptest | 标准库、可校验请求和响应 |

## 1. 命名规范

### 1.1 文件命名

- 测试文件必须以 `_test.go` 结尾。
- 测试文件应与被测试的源文件在同一个包内。
- 测试文件命名格式：`<源文件名>_test.go`。

### 1.2 测试函数命名

- 测试函数必须以 `Test` 开头。
- 格式：`Test<FunctionName>` 或 `Test<StructName>_<MethodName>`。
- 对复杂业务场景使用 `Test<功能名>_<场景描述>`。
- 示例：
  - `TestFormatContext`
  - `TestClient_Search`
  - `TestChat_EmptyMessage`
  - `TestRuntime_ChatNoTextResponse`

### 1.3 辅助函数命名

- 测试辅助函数可以使用小写字母开头。
- 建议使用 `setup`、`teardown`、`create`、`new`、`assert` 等前缀。
- 示例：
  - `setupKnowledgeClient`
  - `newMockMCPClient`
  - `assertChatResponse`

### 1.4 测试文件组织

测试文件的组织结构应该清晰、易于维护。

#### 1.4.1 目录结构

```text
TongjiStudentAgent/
├── internal/
│   ├── integration/knowledge/
│   │   ├── client.go
│   │   ├── client_test.go
│   │   └── testdata/
│   │       ├── search_success.json
│   │       └── search_error.json
│   └── application/chat/
│       ├── service.go
│       ├── service_test.go
│       └── mock/
│           └── runtime_mock.go
└── biz/handler/
    ├── agent.go
    └── agent_test.go
```

#### 1.4.2 测试包的组织

- **包内测试**（推荐）：测试文件与被测试文件使用同一个 package。
  - 优点：可以访问包内的私有成员，适合测试 `withKnowledgeContext`、请求构造等内部协作逻辑。
  - 适用场景：测试包的内部实现细节。
- **包外测试**：测试文件使用 `_test` 后缀的包名。
  - 优点：避免循环依赖，只通过公开 API 验证行为。
  - 适用场景：Hertz Handler、MCP Tool Catalog 等对外契约。

```go
// 包内测试
package knowledge

func TestFormatContext(t *testing.T) {
    // 可以访问包内私有函数和字段
}

// 包外测试
package handler_test

func TestChat(t *testing.T) {
    // 只能访问 handler 的公共 API
}
```

#### 1.4.3 测试辅助文件

- **testdata 目录**：存放 JSON、YAML、MCP Schema、Agent 事件和 HTTP fixture 等测试数据。
  - 命名规范：`<测试名>.<扩展名>`。
  - 不得包含真实 token、Cookie、学生身份、成绩、课表或生产 URL。
- **mock 目录**：存放 `mockgen` 生成的 mock 文件。
  - 命名规范：`<接口名>_mock.go`。

#### 1.4.4 测试文件的组织原则

- 每个源文件对应一个测试文件。
- 测试文件应该与被测试文件在同一目录。
- 复杂的测试可以拆分为多个测试文件。
- 共享的测试辅助函数放在独立的 `helpers_test.go` 中。
- Mock 仅在业务接口稳定后生成，不能为了测试而引入无意义的接口层。

## 2. 测试结构规范

### 2.1 使用 GoConvey 的基本结构

```go
package knowledge

import (
    "testing"

    . "github.com/smartystreets/goconvey/convey"
)

func TestFormatContext(t *testing.T) {
    Convey("知识库上下文格式化", t, func() {
        response := &SearchKnowledgeRes{Data: &SearchKnowledgeData{
            ResultList: []SearchResult{{ChunkTitle: "校园卡", Content: "校园卡可在服务中心办理。"}},
        }}

        Convey("存在有效检索结果", func() {
            result := FormatContext(response)

            Convey("应保留标题和资料内容", func() {
                So(result, ShouldContainSubstring, "校园卡")
                So(result, ShouldContainSubstring, "服务中心")
            })
        })
    })
}
```

### 2.2 嵌套 Convey 结构

GoConvey 支持嵌套结构，便于组织复杂的测试场景：

```go
func TestClientSearch(t *testing.T) {
    Convey("知识库检索测试", t, func() {
        client := setupKnowledgeClient(t)

        Convey("请求成功", func() {
            client.httpClient = newJSONClient(t, http.StatusOK,
                `{"code":0,"data":{"result_list":[{"content":"选课请关注教务处通知"}]}}`)

            Convey("返回检索切片", func() {
                result, err := client.Search(context.Background(), "什么时候选课？")
                So(err, ShouldBeNil)
                So(result.Data.ResultList, ShouldHaveLength, 1)
            })
        })

        Convey("服务返回业务错误", func() {
            client.httpClient = newJSONClient(t, http.StatusOK,
                `{"code":1001,"message":"invalid collection"}`)

            Convey("返回可定位的错误", func() {
                _, err := client.Search(context.Background(), "什么时候选课？")
                So(err, ShouldNotBeNil)
                So(err.Error(), ShouldContainSubstring, "code 1001")
            })
        })
    })
}
```

### 2.4 复杂场景测试

对于复杂的业务场景，需要合理组织测试结构。

#### 2.4.1 测试场景命名

- 使用描述性的测试函数名，明确测试的场景。
- 格式：`Test<功能名>_<场景描述>`。
- 示例：
  - `TestChat`：单轮聊天正常场景。
  - `TestChat_InvalidRequest`：请求体不合法。
  - `TestChat_KnowledgeSearchFailed`：知识库检索失败。
  - `TestToolExecutor_Unauthorized`：身份不满足工具授权要求。

#### 2.4.2 多场景组织

一个测试函数可以包含多个相关场景，使用嵌套 Convey 组织：

```go
func TestChat(t *testing.T) {
    Convey("聊天接口测试", t, func() {
        Convey("正常请求", func() {
            // 设置 fake Runtime 或 mock 应用服务
            // 执行 Handler
            // 验证 200 与 {"message":"..."}
        })

        Convey("请求体不是合法 JSON", func() {
            // 执行 Handler
            // 验证 400 与明确错误信息
        })

        Convey("message 为空白字符", func() {
            // 执行 Handler
            // 验证 400
        })

        Convey("Agent 调用失败", func() {
            // mock Runtime 返回错误
            // 验证 500，且不泄露内部错误详情
        })
    })
}
```

#### 2.4.3 复杂 Mock 场景

对于复杂依赖关系，按调用链从外向内组织 Mock：

```go
func TestCourseScheduleTool(t *testing.T) {
    Convey("课表工具测试", t, func() {
        ctrl := gomock.NewController(t)
        defer ctrl.Finish()

        // 1. 身份上下文 Mock
        mockIdentity := mock.NewMockIdentityProvider(ctrl)
        mockIdentity.EXPECT().CurrentUser(gomock.Any()).Return(TestStudentIdentity(), nil)

        // 2. OpenAPI Client Mock
        mockOpenAPI := mock.NewMockCourseClient(ctrl)
        mockOpenAPI.EXPECT().ListCourses(gomock.Any(), gomock.Any()).
            Return([]Course{{Name: "高等数学", StartAt: "08:00"}}, nil)

        // 3. 领域服务 Mock
        mockService := mock.NewMockCourseService(ctrl)
        mockService.EXPECT().QueryToday(gomock.Any(), gomock.Any()).
            Return([]Course{{Name: "高等数学", StartAt: "08:00"}}, nil)

        // 4. 执行 Tool，并验证输出包含数据时间与适用身份
        result, err := NewCourseTool(mockIdentity, mockService).Invoke(context.Background(), "{}")
        So(err, ShouldBeNil)
        So(result, ShouldContainSubstring, "高等数学")
    })
}
```

真实项目尚未实现课表工具时，上例作为后续 MCP/校园业务 Tool 的组织模板；当前 `get_current_time` 工具只需测试其公开输入输出契约。

#### 2.4.4 Mock 初始化

对于复杂的 Mock 对象，提供初始化函数：

```go
func TestToolExecutor(t *testing.T) {
    Convey("工具执行器测试", t, func() {
        ctrl := gomock.NewController(t)
        defer ctrl.Finish()

        mocks := newToolExecutorMocks(ctrl)
        // 使用 mocks.identity、mocks.policy、mocks.client
    })
}
```

### 2.5 Reset 和 Cleanup

使用 `Reset`、`t.Cleanup` 或 `defer` 进行测试前后的清理：

```go
func TestDefaultService(t *testing.T) {
    Convey("默认聊天服务", t, func() {
        original := defaultService
        defaultService = &Service{}

        Reset(func() {
            defaultService = original
        })

        // 测试逻辑
    })
}
```

包级状态、环境变量、gomonkey patch、MCP Client 和临时资源必须在当前测试结束时恢复或关闭。

## 3. 测试环境配置

### 3.1 环境分类

测试环境分为三种类型，每种环境有不同的用途和要求。

#### 3.1.1 本地环境

- **用途**：开发和调试测试代码。
- **特点**：
  - 所有单元测试应在离线条件下运行。
  - 使用 fake、mock、`httptest`、临时目录和内存实现替代外部依赖。
  - 不读取开发者 `.env` 中的真实凭据。
  - 不创建本地 Sandbox Backend，不执行 Shell。

#### 3.1.2 测试环境

- **用途**：验证 Agent 与测试 MCP Server、Fake OpenAPI 的跨模块行为。
- **特点**：
  - 只使用独立的测试身份和脱敏 fixture。
  - 不使用生产模型、生产知识库或生产校园平台。
  - 适合执行契约测试和集成测试；不作为单元测试的前置条件。

#### 3.1.3 CI 环境

- **用途**：自动化测试和质量检查。
- **特点**：
  - 无人值守，环境变量由 CI 注入。
  - 单元测试不应需要任何密钥或网络访问。
  - 执行覆盖率、竞态检测、超时控制和静态检查。
  - 失败时阻止代码合并。

### 3.2 环境配置管理

使用统一配置管理方式，确保不同环境的一致性：

```go
package testutil

import (
    "os"
    "strings"
)

type TestConfig struct {
    Env     string
    UseMock bool
}

func GetTestConfig() TestConfig {
    env := strings.ToLower(strings.TrimSpace(os.Getenv("TEST_ENV")))
    if env == "" {
        env = "local"
    }
    return TestConfig{Env: env, UseMock: env != "integration"}
}
```

`TEST_ENV` 仅用于测试辅助逻辑，不能改变生产业务的鉴权、工具授权或安全行为。

### 3.3 环境适配

测试代码应适配不同环境：

```go
func TestKnowledgeSearch(t *testing.T) {
    Convey("知识库检索环境适配", t, func() {
        cfg := testutil.GetTestConfig()
        if cfg.UseMock {
            // 使用 httptest 或自定义 RoundTripper
        }

        // 单元测试逻辑始终不访问真实知识库
    })
}
```

### 3.4 环境隔离

确保不同环境的测试互不影响：

- **本地环境**：使用临时文件、内存数据和 mock HTTP Transport。
- **测试环境**：使用独立 namespace、测试账号和可销毁服务实例。
- **CI 环境**：每次运行创建新的临时目录和测试数据，并在结束后清理。

### 3.5 环境切换

```bash
# 本地单元测试
export TEST_ENV=local
go test ./...

# 测试环境的契约/集成测试
export TEST_ENV=integration
go test -tags=integration ./...

# CI 环境
export TEST_ENV=ci
go test -race -coverprofile=coverage.out ./...
```

## 4. GoConvey 断言规范

### 4.1 基本断言

使用 GoConvey 的 `So` 函数进行断言：

```go
import . "github.com/smartystreets/goconvey/convey"

So(actual, ShouldEqual, expected)
So(err, ShouldBeNil)
So(result, ShouldNotBeNil)
So(response.Message, ShouldContainSubstring, "课程")
```

### 4.2 断言组合

```go
Convey("验证聊天响应", func() {
    So(err, ShouldBeNil)
    So(response.StatusCode, ShouldEqual, http.StatusOK)
    So(response.Message, ShouldNotBeBlank)
})
```

Handler 测试至少断言 HTTP 状态码、响应 JSON 和错误分支，不得只断言函数未 panic。

## 5. Gomonkey Mock 规范

### 5.1 基本用法

使用开源 `github.com/agiledragon/gomonkey/v2` 在运行时 mock 函数、方法、私有方法和全局变量：

```go
import (
    "testing"

    "github.com/agiledragon/gomonkey/v2"
    . "github.com/smartystreets/goconvey/convey"
)

func TestWithGomonkey(t *testing.T) {
    Convey("函数 mock", t, func() {
        patches := gomonkey.ApplyFunc(externalFunction, func(arg string) string {
            return "mocked result"
        })
        defer patches.Reset()

        result := functionUnderTest("test input")
        So(result, ShouldEqual, "expected output")
    })
}
```

### 5.2 常用 Mock 方法

- `ApplyFunc`：Mock 函数。
- `ApplyMethod`：Mock 方法。
- `ApplyPrivateMethod`：Mock 私有方法。
- `ApplyGlobalVar`：Mock 全局变量。
- `ApplyFuncSeq`：Mock 函数序列。

### 5.3 注意事项

- 始终使用 `defer patches.Reset()` 清理 mock。
- Mock 的范围尽可能小。
- 避免在同一个测试中 mock 多个函数。
- 对于接口依赖，优先使用 gomock。
- 不得在 `t.Parallel()`、并发测试或会被其他测试共享的包级状态上使用 gomonkey。
- gomonkey 依赖非内联行为时，仅对含 patch 的测试使用 `go test -gcflags=all=-l <package>`；不可将其设为全项目默认命令。

### 5.4 复杂 Mock 场景

#### 5.4.1 多层依赖 Mock

当无法通过接口 mock 的遗留调用链存在多层依赖时，按层次设置 patch，并保证一个 `Patches` 统一清理：

```go
func TestLegacyAdapter(t *testing.T) {
    Convey("遗留适配器测试", t, func() {
        patches := gomonkey.ApplyFunc(loadConfiguration, func() Config {
            return Config{Endpoint: "http://fake.example"}
        }).ApplyFunc(newHTTPClient, func() *http.Client {
            return fakeHTTPClient()
        })
        defer patches.Reset()

        result, err := runLegacyAdapter(context.Background())
        So(err, ShouldBeNil)
        So(result, ShouldNotBeNil)
    })
}
```

如需多层 patch，应优先评估是否可以先改造为构造函数注入；gomonkey 只能作为短期隔离手段。

#### 5.4.2 Mock 初始化

复杂 patch 可由测试辅助函数创建，但辅助函数必须返回清理句柄：

```go
func setupLegacyPatches(t *testing.T) *gomonkey.Patches {
    t.Helper()
    return gomonkey.ApplyFunc(nowUTC, func() time.Time {
        return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
    })
}
```

#### 5.4.3 链式 Mock

使用 gomonkey 的链式调用进行多个紧密相关的 Mock：

```go
patches := gomonkey.ApplyFunc(loadToken, func() string { return "test-token" }).
    ApplyFunc(sendRequest, func(context.Context) error { return nil })
defer patches.Reset()
```

## 6. Gomock Mock 规范

### 6.1 生成 Mock 文件

使用 `mockgen` 工具生成 mock 文件：

```bash
# 为接口生成 mock 文件
mockgen -source=client.go -destination=mock/client_mock.go -package=mock

# 为校园业务接口生成 mock 文件
mockgen -source=internal/integration/campus/course_client.go \
  -destination=internal/integration/campus/mock/course_client_mock.go \
  -package=mock

# 使用公开模块导入路径生成
mockgen github.com/Charlie-BU/TongjiStudent/internal/agentic/tool Executor,Policy > internal/agentic/tool/mock/tool_mock.go
```

### 6.2 基本用法

```go
import (
    "testing"

    "github.com/golang/mock/gomock"
    . "github.com/smartystreets/goconvey/convey"
)

func TestWithGomock(t *testing.T) {
    Convey("使用 gomock 测试", t, func() {
        ctrl := gomock.NewController(t)
        defer ctrl.Finish()

        mockClient := mock.NewMockCourseClient(ctrl)
        mockClient.EXPECT().
            ListCourses(gomock.Any(), gomock.Eq("student-001")).
            Return([]Course{{Name: "高等数学"}}, nil).
            Times(1)

        result, err := FunctionUnderTest(mockClient, "student-001")
        So(err, ShouldBeNil)
        So(result, ShouldHaveLength, 1)
    })
}
```

### 6.3 常用参数匹配器

- `gomock.Any()`：匹配任意值。
- `gomock.Eq(value)`：匹配特定值。
- `gomock.Not(value)`：匹配不等于某值。
- `gomock.Len(n)`：匹配长度。
- `gomock.Nil()`：匹配 nil。

### 6.4 调用次数验证

- `Times(n)`：精确调用 n 次。
- `MinTimes(n)`：至少调用 n 次。
- `MaxTimes(n)`：最多调用 n 次。

### 6.5 注意事项

- 始终使用 `defer ctrl.Finish()` 清理 mock。
- 明确指定期望的参数和返回值。
- 避免过度 mock，只 mock 必要的接口方法。
- 使用 `DoAndReturn` 进行复杂验证。
- 不要 mock Eino、Hertz 或 MCP 库的内部实现；围绕本项目拥有的适配接口断言行为。

## 7. 测试覆盖范围

### 7.1 必须覆盖的场景

- 正常场景：预期的输入和输出。
- 边界场景：空值、零值、最大值、最小值、空检索结果和无工具结果。
- 错误场景：非法 JSON、配置错误、网络错误、超时、上游业务错误和资源关闭失败。
- 并发场景：函数涉及并发、Session、缓存、锁或 shutdown hook 时必须覆盖。
- 安全场景：身份缺失、越权、敏感字段脱敏、Sandbox 默认关闭、不可信知识内容不得成为指令。

当前项目至少覆盖下列模块：

| 模块 | 必测内容 |
| --- | --- |
| `biz/handler` | Ping 响应、Chat 的非法 JSON、空 message、成功、Agent 错误映射 |
| `internal/application/chat` | 默认服务未初始化、知识库直通/命中/失败、Runtime 错误、Close nil 安全 |
| `internal/agentic/runtime` | ChatModel 缺失、Runtime 未初始化、最终 Assistant 文本、无文本、事件错误 |
| `internal/integration/knowledge` | 环境变量、签名请求、HTTP/业务码/JSON 错误、上下文格式化 |
| `internal/integration/mcp` | 本地 Client、Tool Catalog、工具调用、nil Client、RFC3339 UTC 输出 |
| `internal/integration/sandbox` | 开关缺省/真假/非法值；单测不得执行本机 Shell |
| `internal/platform/config` | 端口和优雅退出超时的默认值、覆盖值、非法值 |

### 7.2 使用 GoConvey 组织多场景测试

```go
func TestFunctionCoverage(t *testing.T) {
    Convey("函数测试覆盖", t, func() {
        Convey("正常场景", func() {
            result := Function("normal input")
            So(result, ShouldEqual, "expected output")
        })

        Convey("边界场景 - 空值", func() {
            result := Function("")
            So(result, ShouldEqual, "")
        })

        Convey("边界场景 - 零值", func() {
            result := Function(0)
            So(result, ShouldEqual, "zero value output")
        })

        Convey("错误场景 - 无效输入", func() {
            _, err := Function("invalid")
            So(err, ShouldNotBeNil)
        })
    })
}
```

### 7.3 测试覆盖率目标

- 核心业务逻辑：覆盖率 ≥ 80%。
- 工具函数、配置解析和协议转换：覆盖率 ≥ 90%。
- 整体项目：覆盖率 ≥ 70%。
- 鉴权、工具策略、HITL、数据脱敏和安全开关：正常、拒绝和失败分支必须覆盖，不能只依赖行覆盖率。

## 8. 外部依赖 Mock

### 8.1 Redis Mock

后续引入 Redis Session、锁或 Checkpoint 时，使用开源 `github.com/alicebob/miniredis/v2` 进行 Redis mock；不得连接共享或生产 Redis。

### 8.2 HTTP Mock

使用 `net/http/httptest` 或自定义 `http.RoundTripper` mock Ark 知识库、同济开放平台和远程 MCP HTTP 请求。测试需校验请求方法、URL、必要 Header、请求体、超时和错误响应。

### 8.3 数据库 Mock

后续引入数据库时，使用开源 `github.com/DATA-DOG/go-sqlmock` 或独立临时数据库。不得复用开发库、测试共享库或生产库。

## 9. 资源清理

### 9.1 使用 Reset 进行清理

```go
func TestWithReset(t *testing.T) {
    Convey("测试场景", t, func() {
        file, err := os.CreateTemp("", "tongji_student_test_*")
        So(err, ShouldBeNil)
        Reset(func() {
            _ = file.Close()
            _ = os.Remove(file.Name())
        })

        // 测试逻辑
    })
}
```

### 9.2 使用 defer 进行清理

```go
func TestWithDefer(t *testing.T) {
    Convey("测试场景", t, func() {
        file, err := os.CreateTemp("", "tongji_student_test_*")
        So(err, ShouldBeNil)
        defer func() { _ = file.Close() }()
        defer func() { _ = os.Remove(file.Name()) }()

        // 测试逻辑
    })
}
```

优先使用 `t.Cleanup` 管理 package 级变量、环境变量恢复、临时 Server 和 MCP Client 关闭。

## 10. 并发测试

### 10.1 并发测试

使用 `sync.WaitGroup` 和 channel 进行并发测试。对于后续 Session、Checkpoint、缓存和 tool 调度逻辑，需验证并发请求下的顺序和锁释放。

### 10.2 并发安全测试

确保函数在并发场景下正确，并使用以下命令检查竞态：

```bash
go test -race ./...
```

包含环境变量、包级默认服务、gomonkey patch、共享端口或共享 fixture 的测试不得使用 `t.Parallel()`。

## 11. 性能测试

### 11.1 基准测试

使用 `Benchmark` 函数测试格式化、上下文装配、Schema 校验等可重复的纯逻辑；基准测试不得调用真实模型和网络。

### 11.2 内存分配测试

使用 `testing.AllocsPerRun` 测试高频上下文格式化、事件转换和缓存序列化等路径的内存分配。

## 12. 测试数据管理

### 12.1 测试数据组织

- 简单的测试数据直接在测试函数中定义。
- 复杂的 HTTP JSON、MCP Schema、Agent Event 使用 `testdata` 目录。
- 测试数据文件命名：`<测试名>.<扩展名>`。
- 使用合成学生身份和校园数据，如 `student-001`、`测试校区`；禁止复制真实个人数据。

### 12.2 使用测试数据文件

```go
func TestWithTestData(t *testing.T) {
    Convey("使用测试数据文件", t, func() {
        data, err := os.ReadFile("testdata/search_success.json")
        So(err, ShouldBeNil)

        var input SearchKnowledgeRes
        err = json.Unmarshal(data, &input)
        So(err, ShouldBeNil)

        result := FormatContext(&input)
        So(result, ShouldNotBeBlank)
    })
}
```

## 13. AI 友好性规范

### 13.1 测试函数文档

为复杂测试函数添加清晰注释，说明测试目的、场景、依赖边界和业务安全约束。简单、名称已足够表达意图的测试不必添加重复注释。

### 13.2 结构化测试数据

使用结构化测试数据，便于维护和生成：

```go
type testCase struct {
    name        string
    description string
    message     string
    want        string
    wantErr     bool
    setup       func(*testing.T)
    teardown    func(*testing.T)
}

func TestChatWithStructuredData(t *testing.T) {
    tests := []testCase{
        {name: "正常情况", description: "输入有效校园问题", message: "图书馆在哪里？", want: "图书馆"},
        {name: "错误情况", description: "Runtime 不可用", message: "图书馆在哪里？", wantErr: true},
    }

    Convey("结构化测试数据", t, func() {
        for _, tt := range tests {
            Convey(tt.name, func() {
                if tt.setup != nil {
                    tt.setup(t)
                }
                if tt.teardown != nil {
                    defer tt.teardown(t)
                }

                result, err := FunctionUnderTest(tt.message)
                if tt.wantErr {
                    So(err, ShouldNotBeNil)
                } else {
                    So(err, ShouldBeNil)
                    So(result, ShouldContainSubstring, tt.want)
                }
            })
        }
    })
}
```

## 14. 最佳实践

### 14.1 遵循 FIRST 原则

- **F**ast：测试应快速执行，不等待真实网络或模型。
- **I**ndependent：测试之间相互独立。
- **R**epeatable：测试可重复执行，结果确定。
- **S**elf-validating：测试有明确通过/失败结果。
- **T**imely：实现功能时同步编写测试。

### 14.2 GoConvey 最佳实践

- 使用嵌套 Convey 组织相关测试。
- 使用 Reset 进行测试间清理。
- 使用描述性的 Convey 文本说明业务场景。
- 避免在测试中打印日志，使用 So 断言代替。

### 14.3 Gomonkey 最佳实践

- 始终使用 `defer patches.Reset()` 清理 mock。
- Mock 范围尽可能小。
- 避免在同一个测试中 mock 多个不相关函数。
- 对接口依赖优先使用 gomock。

### 14.4 Gomock 最佳实践

- 始终使用 `defer ctrl.Finish()` 清理 mock。
- 明确指定期望的参数和返回值。
- 避免过度 mock，只 mock 必要接口方法。
- 使用 `DoAndReturn` 进行复杂验证。

### 14.5 避免反模式

- 不要在测试中打印日志。
- 不要使用 `time.Sleep`；使用 channel、context 或 fake clock。
- 不要为测试第三方内部实现而编写脆弱断言。
- 不要硬编码真实环境依赖、真实凭据或学生数据。
- 不要在同一个测试中混合多个不必要的 mock 工具。
- 不要在单元测试中调用真实模型、校园 API、知识库、远程 MCP 或本机 Shell。

### 14.6 代码组织

- 按功能组织测试函数。
- 使用嵌套 Convey 组织相关测试。
- 保持测试函数简洁，每个 Convey 只测试一个方面。
- 将 setup 和 teardown 逻辑放在合适位置。

## 15. CI/CD 集成

### 15.1 运行测试

```bash
# 运行所有单元测试
go test ./...

# 运行指定包的测试
go test ./internal/integration/knowledge

# 运行带覆盖率的测试
go test -coverprofile=coverage.out ./...

# 查看覆盖率报告
go tool cover -html=coverage.out

# 检查竞态和静态问题
go test -race ./...
go vet ./...
```

### 15.2 CI 配置

确保 CI 管道中包含：

- 单元测试执行。
- 代码覆盖率检查。
- 测试超时设置。
- 竞态检测和静态检查。
- 失败测试的详细报告。
- 不向单元测试注入生产凭据或真实校园数据。

## 16. 测试隔离

### 16.1 测试隔离的重要性

测试隔离是确保测试可靠性和可维护性的关键：

- **独立性**：每个测试独立运行，不依赖其他测试的执行顺序。
- **可重复性**：测试可以重复执行，每次结果一致。
- **快速失败**：测试失败应快速定位，而不是被其他测试影响。

### 16.2 隔离原则

#### 16.2.1 数据隔离

- 每个测试使用独立的测试数据。
- 避免共享全局变量。
- 使用临时文件和目录。

```go
func TestDataIsolation(t *testing.T) {
    Convey("数据隔离", t, func() {
        directory := t.TempDir()
        path := filepath.Join(directory, "context.json")
        err := os.WriteFile(path, []byte(`{"question":"测试问题"}`), 0o600)
        So(err, ShouldBeNil)

        data, err := os.ReadFile(path)
        So(err, ShouldBeNil)
        So(string(data), ShouldContainSubstring, "测试问题")
    })
}
```

#### 16.2.2 资源隔离

- 每个测试使用独立资源，例如 `httptest.Server`、临时数据库或 miniredis 实例。
- 测试结束后清理资源。
- 使用 Reset、`t.Cleanup` 或 defer 确保资源清理。

```go
func TestResourceIsolation(t *testing.T) {
    Convey("资源隔离", t, func() {
        server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            _, _ = w.Write([]byte(`{"code":0}`))
        }))
        Reset(server.Close)

        response, err := server.Client().Get(server.URL)
        So(err, ShouldBeNil)
        defer response.Body.Close()
        So(response.StatusCode, ShouldEqual, http.StatusOK)
    })
}
```

#### 16.2.3 Mock 隔离

- 每个 mock 只在需要的测试中使用。
- 使用 defer 确保清理 mock。
- 避免全局 mock。

```go
func TestMockIsolation(t *testing.T) {
    Convey("Mock 隔离", t, func() {
        patches := gomonkey.ApplyFunc(externalFunction, func() string {
            return "mocked"
        })
        defer patches.Reset()

        result := functionUnderTest()
        So(result, ShouldEqual, "mocked")
    })
}
```

### 16.3 避免共享状态

#### 16.3.1 避免全局变量

```go
// 不推荐：使用全局变量
var globalCounter int

func TestWithGlobalVar(t *testing.T) {
    globalCounter = 0
}

// 推荐：使用局部变量
func TestWithLocalVar(t *testing.T) {
    Convey("使用局部变量", t, func() {
        counter := 0
        So(counter, ShouldEqual, 0)
    })
}
```

对于必须替换的包级默认服务或 shutdown hook，必须保存原值并在 `t.Cleanup` 或 Reset 中恢复。

#### 16.3.2 避免共享资源

```go
// 不推荐：共享 HTTP Server 或数据库连接
var sharedServer *httptest.Server

// 推荐：每个测试创建独立资源
func TestWithIndependentServer(t *testing.T) {
    Convey("使用独立 HTTP Server", t, func() {
        server := httptest.NewServer(http.NotFoundHandler())
        defer server.Close()
        So(server.URL, ShouldNotBeBlank)
    })
}
```

### 16.4 测试清理策略

#### 16.4.1 使用 Reset 清理

```go
func TestWithReset(t *testing.T) {
    Convey("测试清理", t, func() {
        var counter int

        Convey("子测试 1", func() {
            counter = 1
            So(counter, ShouldEqual, 1)
        })

        Convey("子测试 2", func() {
            counter = 2
            So(counter, ShouldEqual, 2)
        })

        Reset(func() { counter = 0 })
    })
}
```

#### 16.4.2 使用 defer 清理

```go
func TestWithDefer(t *testing.T) {
    Convey("defer 清理", t, func() {
        server := httptest.NewServer(http.NotFoundHandler())
        defer server.Close()

        // 测试逻辑
    })
}
```

### 16.5 并发测试隔离

并发测试需要特别注意隔离：

```go
func TestConcurrentIsolation(t *testing.T) {
    Convey("并发测试隔离", t, func() {
        var wg sync.WaitGroup
        results := make(chan int, 10)

        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(id int) {
                defer wg.Done()
                results <- ConcurrentFunction(id)
            }(i)
        }

        wg.Wait()
        close(results)

        count := 0
        for range results {
            count++
        }
        So(count, ShouldEqual, 10)
    })
}
```

### 16.6 测试隔离检查清单

- [ ] 每个测试是否独立运行？
- [ ] 测试之间是否没有共享状态？
- [ ] 测试是否可以任意顺序执行？
- [ ] 测试是否可以重复执行？
- [ ] 测试结束后是否清理了所有资源？
- [ ] Mock 是否在测试结束后清理？
- [ ] 临时文件是否在测试结束后删除？
- [ ] HTTP/MCP Client、数据库连接是否在测试结束后关闭？

## 17. 测试重构

### 17.1 测试重构的重要性

测试重构是保持测试代码质量和可维护性的关键：

- **提高可读性**：清晰的测试代码更容易理解和维护。
- **减少重复**：消除重复代码，提高复用性。
- **提高可维护性**：易于修改和扩展测试。
- **提高执行效率**：优化测试结构，减少执行时间。

### 17.2 重构原则

#### 17.2.1 DRY 原则（Don't Repeat Yourself）

避免重复 setup 代码，提取公共逻辑：

```go
func setupTestHTTPClient(t *testing.T, status int, body string) *http.Client {
    t.Helper()
    return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
        return &http.Response{
            StatusCode: status,
            Body:       io.NopCloser(strings.NewReader(body)),
            Header:     make(http.Header),
        }, nil
    })}
}

func TestSearchSuccess(t *testing.T) {
    client := setupTestHTTPClient(t, http.StatusOK, `{"code":0}`)
    _ = client
}

func TestSearchFailure(t *testing.T) {
    client := setupTestHTTPClient(t, http.StatusBadGateway, `unavailable`)
    _ = client
}
```

#### 17.2.2 单一职责原则

每个测试函数只测试一个功能点：

```go
func TestChatRejectsEmptyMessage(t *testing.T) {
    // 只验证空 message 的 400 行为
}

func TestChatMapsRuntimeError(t *testing.T) {
    // 只验证 Runtime 错误的 500 映射
}
```

#### 17.2.3 测试数据驱动

使用测试数据驱动测试，减少重复代码：

```go
func TestEnabledFromEnv(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    bool
        wantErr bool
    }{
        {name: "空值", input: "", want: false},
        {name: "开启", input: "true", want: true},
        {name: "非法值", input: "enabled", wantErr: true},
    }

    Convey("Sandbox 开关验证", t, func() {
        for _, tt := range tests {
            Convey(tt.name, func() {
                t.Setenv("SANDBOX_ENABLED", tt.input)
                result, err := EnabledFromEnv()
                if tt.wantErr {
                    So(err, ShouldNotBeNil)
                } else {
                    So(err, ShouldBeNil)
                    So(result, ShouldEqual, tt.want)
                }
            })
        }
    })
}
```

### 17.3 重构技巧

#### 17.3.1 提取测试辅助函数

```go
func createTestStudent(id, campus string) StudentIdentity {
    return StudentIdentity{ID: id, Campus: campus}
}

func assertCourseEqual(t *testing.T, got, want Course) {
    t.Helper()
    So(got.Name, ShouldEqual, want.Name)
    So(got.StartAt, ShouldEqual, want.StartAt)
}
```

#### 17.3.2 使用测试构建器

```go
type SearchResultBuilder struct {
    result SearchResult
}

func NewSearchResultBuilder() *SearchResultBuilder {
    return &SearchResultBuilder{}
}

func (b *SearchResultBuilder) WithTitle(title string) *SearchResultBuilder {
    b.result.ChunkTitle = title
    return b
}

func (b *SearchResultBuilder) WithContent(content string) *SearchResultBuilder {
    b.result.Content = content
    return b
}

func (b *SearchResultBuilder) Build() SearchResult {
    return b.result
}
```

#### 17.3.3 使用表驱动测试

```go
func TestTableDriven(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {name: "正常情况", input: "input", expected: "output"},
        {name: "错误情况", input: "invalid", wantErr: true},
    }

    Convey("表驱动测试", t, func() {
        for _, tt := range tests {
            Convey(tt.name, func() {
                result, err := Function(tt.input)
                if tt.wantErr {
                    So(err, ShouldNotBeNil)
                } else {
                    So(err, ShouldBeNil)
                    So(result, ShouldEqual, tt.expected)
                }
            })
        }
    })
}
```

### 17.4 重构最佳实践

#### 17.4.1 保持测试的独立性

重构时不要让测试依赖其他测试的状态；每个 case 自行构造数据、服务和 mock。

#### 17.4.2 保持测试的清晰性

重构时不要为消除少量重复而过度抽象。测试应该能直接看出输入、依赖、动作和断言。

#### 17.4.3 保持测试的完整性

重构后必须保留正常、边界和错误场景；对鉴权、Tool Gate、HITL、知识库注入和数据脱敏，必须额外保留安全拒绝分支。

### 17.5 重构检查清单

- [ ] 是否消除了重复代码？
- [ ] 是否提高了测试的可读性？
- [ ] 是否保持了测试的独立性？
- [ ] 是否保持了测试的完整性？
- [ ] 是否保持了测试的覆盖率？
- [ ] 是否保持了测试的执行速度？
- [ ] 是否通过了所有测试？
- [ ] 是否添加了必要的注释？

## 附录：常用测试工具

### A.1 测试框架

- `testing`：Go 标准测试包。
- `goconvey`：BDD 风格的测试框架。

### A.2 Mock 工具

- `gomonkey`：运行时 mock 工具，使用开源 `github.com/agiledragon/gomonkey/v2`。
- `gomock`：接口 mock 工具。
- `miniredis`：Redis mock。
- `sqlmock`：SQL mock。
- `httptest`：HTTP 测试。

### A.3 覆盖率工具

- `go test -cover`：内置覆盖率。
- `gocov`：覆盖率工具。
- `gocov-html`：HTML 覆盖率报告。

### A.4 性能分析

- `go test -bench`：基准测试。
- `pprof`：性能分析。
