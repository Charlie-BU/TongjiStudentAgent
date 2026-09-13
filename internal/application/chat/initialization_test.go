package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/runtime"
	sessionconfig "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/config"
	sessionpostgres "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/postgres"
	sessionredis "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/store/redis"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/webfetch"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	mcpclient "github.com/mark3labs/mcp-go/client"
	. "github.com/smartystreets/goconvey/convey"
)

// TestNewFromEnvResourceOwnership 覆盖启动失败清理与成功后的资源所有权转移。
func TestNewFromEnvResourceOwnership(t *testing.T) {
	t.Setenv("PRO_MODEL", "")
	t.Setenv("MAX_MODEL", "")
	Convey("启动各阶段失败只关闭已经取得的资源", t, func() {
		for _, tc := range []struct {
			stage  string
			closed []string
		}{
			{"instruction", nil}, {"model", nil}, {"knowledge", nil}, {"catalog", nil},
			{"tavily", nil}, {"webfetch", nil}, {"mcp", nil},
			{"mcpTools", []string{"mcp"}}, {"sandboxEnabled", []string{"mcp"}},
			{"middleware", []string{"mcp"}}, {"sessionConfig", []string{"mcp"}},
			{"postgres", []string{"mcp"}}, {"schema", []string{"postgres", "mcp"}},
			{"redis", []string{"postgres", "mcp"}},
			{"taskplan", []string{"redis", "postgres", "mcp"}},
			{"runtime", []string{"redis", "postgres", "mcp"}},
		} {
			Convey(tc.stage, func() {
				fixture := newInitializationFixture(tc.stage)
				service, err := newFromEnv(context.Background(), fixture.deps)
				So(service, ShouldBeNil)
				So(errors.Is(err, fixture.failure), ShouldBeTrue)
				So(fixture.closed, ShouldResemble, tc.closed)
				So(fixture.stages[len(fixture.stages)-1], ShouldEqual, tc.stage)
			})
		}
	})
	Convey("成功后由 Service.Close 释放连接，清理错误不阻断其余资源", t, func() {
		fixture := newInitializationFixture("")
		service, err := newFromEnv(context.Background(), fixture.deps)
		So(err, ShouldBeNil)
		So(service, ShouldNotBeNil)
		So(fixture.closed, ShouldBeEmpty)
		So(service.mcpClient, ShouldEqual, fixture.mcp)
		So(service.postgresSessionStore, ShouldEqual, fixture.postgres)
		So(service.redisSessionStore, ShouldEqual, fixture.redis)
		So(service.historyMessageLimit, ShouldEqual, 7)
		So(service.runtime, ShouldEqual, fixture.runtime)
		err = service.Close()
		So(fixture.closed, ShouldResemble, []string{"redis", "postgres", "mcp"})
		So(errors.Is(err, fixture.closeFailure), ShouldBeTrue)
	})
}

// TestOptionalChromium 验证浏览器不可用时继续初始化且保留调用方取消语义。
func TestOptionalChromium(t *testing.T) {
	t.Setenv("PRO_MODEL", "")
	t.Setenv("MAX_MODEL", "")
	Convey("浏览器为可选启动能力", t, func() {
		fixture := newInitializationFixture("verifyChromium")
		service, err := newFromEnv(context.Background(), fixture.deps)
		if err != nil || service == nil {
			t.Fatalf("optional browser blocked startup: %v", err)
		}
		_ = service.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fixture.deps.verifyChromium = func(context.Context) error { cancel(); return context.Canceled }
		service, err = newFromEnv(ctx, fixture.deps)
		if service != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected startup cancellation, got %v", err)
		}
	})
}

// initializationFixture 完全隔离外部服务，记录实际初始化路径和资源释放次序。
type initializationFixture struct {
	runtime               *recordingSessionRuntime
	deps                  initializationDeps
	stages, closed        []string
	failure, closeFailure error
	mcp                   *mcpclient.Client
	postgres              *sessionpostgres.PostgresStore
	redis                 *sessionredis.RedisEphemeralStore
}

func newInitializationFixture(failStage string) *initializationFixture {
	f := &initializationFixture{runtime: &recordingSessionRuntime{}, failure: errors.New("initialization failed"), closeFailure: errors.New("close failed"), mcp: &mcpclient.Client{}, postgres: &sessionpostgres.PostgresStore{}, redis: &sessionredis.RedisEphemeralStore{}}
	step := func(stage string) error {
		f.stages = append(f.stages, stage)
		if stage == failStage {
			return f.failure
		}
		return nil
	}
	f.deps = initializationDeps{
		instruction:    func(context.Context) (string, error) { return "instruction", step("instruction") },
		model:          func(context.Context, string) (model.BaseChatModel, error) { return nil, step("model") },
		knowledge:      func() (*knowledge.Client, error) { return nil, step("knowledge") },
		catalog:        func() (string, error) { return "catalog", step("catalog") },
		tavily:         func() (*tavily.Client, error) { return nil, step("tavily") },
		webfetch:       func() (*webfetch.Client, error) { return nil, step("webfetch") },
		verifyChromium: func(context.Context) error { return step("verifyChromium") },
		mcp: func(context.Context) (*mcpclient.Client, error) {
			if err := step("mcp"); err != nil {
				return nil, err
			}
			return f.mcp, nil
		},
		mcpTools: func(_ context.Context, client *mcpclient.Client, _ ...string) ([]tool.BaseTool, error) {
			So(client, ShouldEqual, f.mcp)
			return nil, step("mcpTools")
		},
		sandboxEnabled: func() (bool, error) { return true, step("sandboxEnabled") },
		middleware:     func(context.Context) (adk.ChatModelAgentMiddleware, error) { return nil, step("middleware") },
		sessionConfig: func() (sessionconfig.Config, error) {
			return sessionconfig.Config{AnonymousTTL: time.Hour, AnonymousMessageLimit: 9, HistoryMessageLimit: 7}, step("sessionConfig")
		},
		postgres: func(context.Context) (*sessionpostgres.PostgresStore, error) {
			if err := step("postgres"); err != nil {
				return nil, err
			}
			return f.postgres, nil
		},
		schema: func(_ context.Context, p *sessionpostgres.PostgresStore) error {
			So(p, ShouldEqual, f.postgres)
			return step("schema")
		},
		redis: func(_ context.Context, ttl time.Duration, limit int) (*sessionredis.RedisEphemeralStore, error) {
			So(ttl, ShouldEqual, time.Hour)
			So(limit, ShouldEqual, 9)
			if err := step("redis"); err != nil {
				return nil, err
			}
			return f.redis, nil
		},
		taskplan: func(p *sessionpostgres.PostgresStore, r *sessionredis.RedisEphemeralStore) (taskplan.TaskPlanRepository, error) {
			So(p, ShouldEqual, f.postgres)
			So(r, ShouldEqual, f.redis)
			return nil, step("taskplan")
		},
		runtime: func(_ context.Context, cfg runtime.Config) (sessionRuntime, error) {
			So(cfg.Instruction, ShouldEqual, "instruction")
			So(cfg.SkillCatalog, ShouldEqual, "catalog")
			So(cfg.Tools, ShouldNotBeEmpty)
			return f.runtime, step("runtime")
		},
		closeMCP: func(c *mcpclient.Client) error {
			So(c, ShouldEqual, f.mcp)
			f.closed = append(f.closed, "mcp")
			return nil
		},
		closePostgres: func(p *sessionpostgres.PostgresStore) {
			So(p, ShouldEqual, f.postgres)
			f.closed = append(f.closed, "postgres")
		},
		closeRedis: func(r *sessionredis.RedisEphemeralStore) error {
			So(r, ShouldEqual, f.redis)
			f.closed = append(f.closed, "redis")
			return f.closeFailure
		},
	}
	return f
}

func TestTierRuntimeInitialization(t *testing.T) {
	t.Setenv("LITE_MODEL", "model-lite")
	t.Setenv("PRO_MODEL", "model-pro")
	t.Setenv("MAX_MODEL", "model-max")
	Convey("为每档创建 Runtime 并共享工具，资源只释放一次", t, func() {
		f := newInitializationFixture("")
		var ids []string
		f.deps.model = func(_ context.Context, id string) (model.BaseChatModel, error) {
			ids = append(ids, id)
			return nil, nil
		}
		original := f.deps.runtime
		var configs []runtime.Config
		f.deps.runtime = func(ctx context.Context, cfg runtime.Config) (sessionRuntime, error) {
			configs = append(configs, cfg)
			return original(ctx, cfg)
		}
		service, err := newFromEnv(context.Background(), f.deps)
		So(err, ShouldBeNil)
		So(ids, ShouldResemble, []string{"model-lite", "model-pro", "model-max"})
		So(configs, ShouldHaveLength, 3)
		So(service.runtimes, ShouldHaveLength, 3)
		for _, tier := range []string{"lite", "pro", "max"} {
			So(service.runtimes[tier].modelID, ShouldEqual, "model-"+tier)
		}
		So(&configs[0].Tools[0], ShouldEqual, &configs[1].Tools[0])
		So(&configs[0].Tools[0], ShouldEqual, &configs[2].Tools[0])
		_ = service.Close()
		So(f.closed, ShouldResemble, []string{"redis", "postgres", "mcp"})
	})
	Convey("可选模型创建失败会清理已打开资源", t, func() {
		f := newInitializationFixture("")
		f.deps.model = func(_ context.Context, id string) (model.BaseChatModel, error) {
			if id == "model-lite" {
				return nil, nil
			}
			return nil, f.failure
		}
		service, err := newFromEnv(context.Background(), f.deps)
		So(service, ShouldBeNil)
		So(errors.Is(err, f.failure), ShouldBeTrue)
		So(f.closed, ShouldResemble, []string{"redis", "postgres", "mcp"})
	})
}
