package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/joho/godotenv"

	"github.com/Charlie-BU/TongjiStudent/agent"
	"github.com/Charlie-BU/TongjiStudent/integration/fornax"
	logs "github.com/Charlie-BU/TongjiStudent/pkg/logging"
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

	if err := agent.InitDeepAgentAndMcpClient(ctx); err != nil {
		panic(fmt.Errorf("error initializing deep agent client, err: %v", err))
	}

	RegisterShutdownHook(func(ctx context.Context) {
		logs.CtxInfo(ctx, "closing mcp client")
		if err := agent.CloseMcpClient(); err != nil {
			logs.CtxError(ctx, "failed to close mcp client: %v", err)
		}
	})

	logs.Infof("end of initializeClient run")
}
