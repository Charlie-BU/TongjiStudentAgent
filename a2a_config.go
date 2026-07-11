package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/server"

	"code.byted.org/gopkg/logs/v2"
	bagent "code.byted.org/inf/bytedai-go/agent"
)

// buildA2AServerConfig constructs the A2A server configuration including agent card, handlers, and task management.
func buildA2AServerConfig(ctx context.Context) *server.Config {
	psm := os.Getenv("TCE_PSM")
	taskStore, err := bagent.NewBytedTaskStore()
	if err != nil {
		logs.CtxError(ctx, "NewBytedTaskStore %v", err)
		taskStore = nil
	}
	taskLocker, err := bagent.NewBytedTaskLocker()
	if err != nil {
		logs.CtxError(ctx, "NewBytedTaskLocker %v", err)
		taskLocker = nil
	}
	return &server.Config{
		AgentCardConfig:         buildAgentCardConfig(psm),
		MessageHandler:          handleMessage,
		MessageStreamingHandler: handleMessageStreaming,
		CancelTaskHandler:       handleCancelTask,
		TaskEventsConsolidator:  consolidateTaskEvents,
		Logger: func(ctx context.Context, format string, v ...any) {
			logs.CtxInfo(ctx, format, v...)
		},
		TaskStore:  taskStore,
		TaskLocker: taskLocker,
	}
}

// buildAgentCardConfig constructs the agent card configuration for A2A protocol.
func buildAgentCardConfig(psm string) server.AgentCardConfig {
	return server.AgentCardConfig{
		Name:             fmt.Sprintf("%s demo agent", psm),
		Description:      "a deep agent used for testing",
		URL:              "https://127.0.0.1:8080",
		Version:          "1",
		DocumentationURL: "https://example.documentation.com",
		Provider: &models.AgentProvider{
			Organization: "jack",
			URL:          "https://example.com",
		},
		Skills: []models.AgentSkill{
			{
				ID:          "weather-forecast",
				Name:        "weather forecast inquiry",
				Description: ptrOf("It provides real-time weather, forecasts for the next 7 days, and weather warning information for cities around the world, and supports key meteorological data query such as temperature, precipitation, and wind"),
				Tags:        []string{"weather"},
				Examples: []string{
					"How is the weather in Beijing today?",
					"Will it rain in Shanghai in the next three days?",
				},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}
}

// consolidateTaskEvents merges response events into a consolidated task content.
//
// Since eino-ext/a2a v0.0.1-alpha.2 the TaskEventsConsolidator signature changed:
// the task is now reached via params.Task, and handleErr carries the error (if any)
// returned by the streaming handler — the consolidator is now always invoked, even
// when the handler failed.
func consolidateTaskEvents(_ context.Context, params *server.InputParams, events []models.ResponseEvent, handleErr error) *models.TaskContent {
	t := params.Task
	result := &models.TaskContent{
		Status:    t.Status,
		Artifacts: make([]*models.Artifact, len(t.Artifacts)),
		History:   make([]*models.Message, len(t.History)),
		Metadata:  t.Metadata,
	}
	copy(result.Artifacts, t.Artifacts)
	copy(result.History, t.History)

	archiveStatusMessage := func() {
		if result.Status.Message != nil {
			result.History = append(result.History, result.Status.Message)
		}
	}

	for _, e := range events {
		if e.TaskContent != nil {
			archiveStatusMessage()
			result.Status = e.TaskContent.Status
			result.Metadata = e.TaskContent.Metadata
			result.Artifacts = append(result.Artifacts, e.TaskContent.Artifacts...)
			result.History = append(result.History, e.TaskContent.History...)
		} else if e.TaskStatusUpdateEventContent != nil {
			archiveStatusMessage()
			result.Status = e.TaskStatusUpdateEventContent.Status
		} else if e.TaskArtifactUpdateEventContent != nil {
			if e.TaskArtifactUpdateEventContent.Append && len(result.Artifacts) > 0 {
				last := result.Artifacts[len(result.Artifacts)-1]
				last.Parts = append(last.Parts, e.TaskArtifactUpdateEventContent.Artifact.Parts...)
			} else {
				result.Artifacts = append(result.Artifacts, &e.TaskArtifactUpdateEventContent.Artifact)
			}
		}
	}

	// When the streaming handler itself failed, mark the consolidated task as failed.
	if handleErr != nil {
		archiveStatusMessage()
		result.Status = models.TaskStatus{
			State:   models.TaskStateFailed,
			Message: newAgentTextMessage(fmt.Sprintf("task failed: %v", handleErr)),
		}
	}
	return result
}
