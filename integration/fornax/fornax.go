// Package fornax isolates the optional internal Fornax integration.
package fornax

import (
	"context"
	"fmt"
	"os"
	"strings"

	fornaxcallback "code.byted.org/flow/eino-byted-ext/callbacks/fornax"
	"code.byted.org/flowdevops/fornax_sdk"
	"code.byted.org/flowdevops/fornax_sdk/domain"
	"github.com/cloudwego/eino/callbacks"

	logs "github.com/Charlie-BU/TongjiStudent/pkg/logging"
)

// RegisterShutdownHook registers cleanup work for application shutdown.
type RegisterShutdownHook func(func(context.Context))

// Enabled reports whether the optional Fornax integration is explicitly enabled.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FORNAX_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Init creates the Fornax client and registers its callback handler when enabled.
func Init(ctx context.Context, registerShutdownHook RegisterShutdownHook) error {
	if !Enabled() {
		logs.CtxInfo(ctx, "Fornax integration is disabled")
		return nil
	}

	ak, sk := os.Getenv("FORNAX_AK"), os.Getenv("FORNAX_SK")
	if ak == "" || sk == "" {
		return fmt.Errorf("FORNAX_AK and FORNAX_SK must be set when FORNAX_ENABLED is enabled")
	}

	fornaxClient, err := fornax_sdk.NewClient(&domain.Config{
		Identity: &domain.Identity{AK: ak, SK: sk},
	})
	if err != nil {
		return err
	}

	if registerShutdownHook != nil {
		registerShutdownHook(func(ctx context.Context) {
			logs.CtxInfo(ctx, "closing Fornax client")
			fornaxClient.Close(ctx)
		})
	}

	callbacks.AppendGlobalHandlers(fornaxcallback.NewDefaultCallbackHandler(fornaxClient))
	return nil
}
