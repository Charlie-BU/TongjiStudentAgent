package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/joho/godotenv"

	einoext "code.byted.org/flow/eino-byted-ext/byted"
	"code.byted.org/flow/eino-byted-ext/callbacks/fornax"
	"code.byted.org/flowdevops/fornax_sdk"
	"code.byted.org/flowdevops/fornax_sdk/domain"
	"code.byted.org/gopkg/logs/v2"

	"code.byted.org/bytefaas/bytefaas_native_hertz_a2a_deep_agent_demo/agent"
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

	err = einoext.Init()
	if err != nil {
		panic(fmt.Errorf("error initializing eino byted env, err: %v", err))
	}

	if err := initFornaxCallback(); err != nil {
		panic(fmt.Errorf("error initializing fornax callback, err: %v", err))
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

func initFornaxCallback() error {
	if os.Getenv("FORNAX_AK") == "" && os.Getenv("FORNAX_SK") == "" {
		return nil
	}

	identity := &domain.Identity{
		AK: os.Getenv("FORNAX_AK"),
		SK: os.Getenv("FORNAX_SK"),
	}

	fornaxClient, err := fornax_sdk.NewClient(&domain.Config{Identity: identity})
	if err != nil {
		return err
	}

	RegisterShutdownHook(func(ctx context.Context) {
		logs.CtxInfo(ctx, "closing fornax client")
		fornaxClient.Close(ctx)
	})

	handler := fornax.NewDefaultCallbackHandler(fornaxClient)
	callbacks.AppendGlobalHandlers(handler)

	return nil
}
