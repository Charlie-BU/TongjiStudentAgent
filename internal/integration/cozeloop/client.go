package cozeloop

import (
	"context"
	"fmt"
	"os"
	"strings"

	cozeloopcallback "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/cloudwego/eino/callbacks"
	cozeloopsdk "github.com/coze-dev/cozeloop-go"

	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
)

// RegisterShutdownHook 注册关闭钩子
type RegisterShutdownHook func(func(context.Context))

var client cozeloopsdk.Client

// Enabled 表示是否启用了 Cozeloop 集成
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COZELOOP_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Init 初始化 Cozeloop 客户端并注册回调处理程序
func Init(ctx context.Context, registerShutdownHook RegisterShutdownHook) error {
	if !Enabled() {
		logs.CtxInfo(ctx, "Cozeloop integration is disabled")
		return nil
	}

	if os.Getenv("COZELOOP_WORKSPACE_ID") == "" || os.Getenv("COZELOOP_JWT_OAUTH_CLIENT_ID") == "" || os.Getenv("COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID") == "" || os.Getenv("COZELOOP_JWT_OAUTH_PRIVATE_KEY") == "" {
		return fmt.Errorf("COZELOOP_WORKSPACE_ID, COZELOOP_JWT_OAUTH_CLIENT_ID, COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID, and COZELOOP_JWT_OAUTH_PRIVATE_KEY must be set when COZELOOP_ENABLED is enabled")
	}

	newClient, err := cozeloopsdk.NewClient()
	if err != nil {
		return fmt.Errorf("create Cozeloop client: %w", err)
	}
	client = newClient

	if registerShutdownHook != nil {
		registerShutdownHook(func(ctx context.Context) {
			logs.CtxInfo(ctx, "closing Cozeloop client")
			client.Close(ctx)
		})
	}

	callbacks.AppendGlobalHandlers(cozeloopcallback.NewLoopHandler(client))
	logs.CtxInfo(ctx, "Cozeloop integration initialized")
	return nil
}
