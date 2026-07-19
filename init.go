package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/joho/godotenv"

	"github.com/Charlie-BU/TongjiStudent/internal/application/chat"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/fornax"
	logs "github.com/Charlie-BU/TongjiStudent/internal/platform/observability/logging"
)

type ShutdownHook = func(ctx context.Context)

var (
	shutdownHooksMu sync.Mutex
	shutdownHooks   []ShutdownHook
)

// RegisterShutdownHook appends a hook function that will be called during graceful shutdown.
func RegisterShutdownHook(hook ShutdownHook) {
	shutdownHooksMu.Lock()
	defer shutdownHooksMu.Unlock()
	shutdownHooks = append(shutdownHooks, hook)
}

// GetShutdownHooks returns all registered shutdown hooks.
func GetShutdownHooks() []ShutdownHook {
	shutdownHooksMu.Lock()
	defer shutdownHooksMu.Unlock()
	hooks := make([]ShutdownHook, len(shutdownHooks))
	copy(hooks, shutdownHooks)
	return hooks
}

// initializeClient will be called once when the function is initialized
func initializeClient(ctx context.Context) {
	logs.Infof("start to run initializeClient")
	defer logs.Flush()

	err := godotenv.Load()
	if err != nil {
		panic(fmt.Errorf("error loading .env file, err: %v", err))
	}

	if err := fornax.Init(ctx, RegisterShutdownHook); err != nil {
		panic(fmt.Errorf("error initializing Fornax integration, err: %v", err))
	}

	if err := chat.Init(ctx); err != nil {
		panic(fmt.Errorf("error initializing chat service, err: %v", err))
	}

	RegisterShutdownHook(func(ctx context.Context) {
		logs.CtxInfo(ctx, "closing chat service")
		if err := chat.Close(); err != nil {
			logs.CtxError(ctx, "failed to close chat service: %v", err)
		}
	})

	logs.Infof("end of initializeClient run")
}
