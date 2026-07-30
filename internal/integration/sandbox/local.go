// TODO：使用 Ark AgentKit 远程沙箱代替本地沙箱
// Package sandbox 封装 Agent 使用的沙箱能力适配。
package sandbox

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// EnabledFromEnv 读取 SANDBOX_ENABLED 开关。
func EnabledFromEnv() (bool, error) {
	value := strings.TrimSpace(os.Getenv("SANDBOX_ENABLED"))
	if value == "" {
		return false, nil
	}

	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse SANDBOX_ENABLED: %w", err)
	}
	return enabled, nil
}

// NewFileSystemMiddleware 创建基于本地 Backend 的文件系统中间件。
func NewFileSystemMiddleware(ctx context.Context) (adk.ChatModelAgentMiddleware, error) {
	localSandbox, err := newLocalSandbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("create local sandbox: %w", err)
	}
	return filesystem.New(ctx, &filesystem.MiddlewareConfig{
		Backend:        localSandbox,
		StreamingShell: localSandbox,
	})
}

// newLocalSandbox 创建本地沙箱 Backend。
func newLocalSandbox(ctx context.Context) (*local.Local, error) {
	return local.NewBackend(ctx, &local.Config{})
}
