package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/server"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"code.byted.org/gopkg/logs/v2"

	"code.byted.org/bytefaas/bytefaas_native_hertz_a2a_deep_agent_demo/agent"
)

const toolResultFooter = "\n---\n"

func toolResultHeader(toolName string) string {
	return fmt.Sprintf("\n---\n📊 Tool Return Result:\nTool Name: %s , Result :", toolName)
}

func newAgentTextMessage(content string) *models.Message {
	return &models.Message{
		Role: models.RoleAgent,
		Parts: []models.Part{{
			Kind: models.PartKindText,
			Text: ptrOf(content),
		}},
	}
}

func newFailedTaskContent(task *models.Task, err error) *models.TaskContent {
	return newTaskContent(task, models.TaskStateFailed, newAgentTextMessage(fmt.Sprintf("task failed: %v", err)))
}

func newTaskContent(task *models.Task, state models.TaskState, message *models.Message) *models.TaskContent {
	return &models.TaskContent{
		Status: models.TaskStatus{
			State:   state,
			Message: message,
		},
		History:   task.History,
		Artifacts: task.Artifacts,
		Metadata:  task.Metadata,
	}
}

// extractUserMessages extracts text messages from A2A input parts and converts them to schema.Message.
func extractUserMessages(input *models.Message) []*schema.Message {
	userMessages := make([]*schema.Message, 0, len(input.Parts))
	for _, p := range input.Parts {
		if p.Kind == models.PartKindText && p.Text != nil {
			userMessages = append(userMessages, schema.UserMessage(*p.Text))
		}
	}
	return userMessages
}

// handleMessage handles a non-streaming A2A message request.
func handleMessage(ctx context.Context, params *server.InputParams) (*models.TaskContent, error) {
	userMessages := extractUserMessages(params.Input)
	runner, err := agent.GetRunner(ctx, false, nil)
	if err != nil {
		logs.CtxError(ctx, "GetDeepAgent %v", err)
		return newFailedTaskContent(params.Task, err), nil
	}

	eventIter := runner.Run(ctx, userMessages)
	sb := strings.Builder{}
	for {
		event, ok := eventIter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			logs.CtxError(ctx, "agent Run failed, err: %v", event.Err)
			return newFailedTaskContent(params.Task, event.Err), nil
		}
		if content := formatAgentOut(event.Output); content != "" {
			sb.WriteString(content)
		}
	}

	return newTaskContent(params.Task, models.TaskStateCompleted, newAgentTextMessage(sb.String())), nil
}

// handleMessageStreaming handles a streaming A2A message request.
func handleMessageStreaming(ctx context.Context, params *server.InputParams, writer server.ResponseEventWriter) error {
	userMessages := extractUserMessages(params.Input)
	runner, err := agent.GetRunner(ctx, true, nil)
	if err != nil {
		logs.CtxError(ctx, "GetDeepAgent %v", err)
		return writeFailedEvent(writer, err)
	}
	eventIter := runner.Run(ctx, userMessages)

	if err := writeInitialTaskState(writer, params.Task); err != nil {
		logs.CtxError(ctx, "write response failed, err: %v", err)
		return err
	}

	for {
		event, ok := eventIter.Next()
		if !ok {
			return writeCompletedEvent(writer)
		}
		if event.Err != nil {
			logs.CtxError(ctx, "agent Run stream failed, err: %v", event.Err)
			return writeFailedEvent(writer, event.Err)
		}
		if err := processAgentOutput(writer, event.Output); err != nil {
			logs.CtxError(ctx, "process stream output failed, err: %v", err)
			return writeFailedEvent(writer, err)
		}
	}
}

// handleCancelTask handles an A2A task cancellation request.
func handleCancelTask(_ context.Context, params *server.InputParams) (*models.TaskContent, error) {
	return newTaskContent(params.Task, models.TaskStateCanceled, nil), nil
}

func writeInitialTaskState(writer server.ResponseEventWriter, task *models.Task) error {
	return writer.Write(models.ResponseEvent{TaskContent: &models.TaskContent{
		Status:    task.Status,
		Artifacts: task.Artifacts,
		History:   task.History,
		Metadata:  task.Metadata,
	}})
}

func writeCompletedEvent(writer server.ResponseEventWriter) error {
	return writer.Write(models.ResponseEvent{
		TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
			Status: models.TaskStatus{
				State:   models.TaskStateCompleted,
				Message: nil,
			},
			Final: true,
		},
	})
}

func writeFailedEvent(writer server.ResponseEventWriter, err error) error {
	return writer.Write(models.ResponseEvent{
		TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
			Status: models.TaskStatus{
				State:   models.TaskStateFailed,
				Message: newAgentTextMessage(fmt.Sprintf("task failed: %v", err)),
			},
			Final: true,
		},
	})
}

func formatToolResult(toolName, content string) string {
	return toolResultHeader(toolName) + content + toolResultFooter
}

func formatAgentOut(output *adk.AgentOutput) string {
	if output == nil || output.MessageOutput == nil {
		return ""
	}
	msgOut := output.MessageOutput
	msg, err := msgOut.GetMessage()
	if err != nil || msg == nil {
		return ""
	}
	content := msg.Content
	if content == "" {
		return ""
	}
	if msgOut.Role == schema.Tool {
		return formatToolResult(msgOut.ToolName, content)
	}
	return content
}

func streamMessageToWriter(writer server.ResponseEventWriter, msgOut *adk.MessageVariant) error {
	if msgOut.MessageStream != nil {
		for {
			message, err := msgOut.MessageStream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("receive message failed: %w", err)
			}
			if err := writeToContent(writer, message.Content); err != nil {
				return err
			}
		}
	}
	if msgOut.Message != nil {
		return writeToContent(writer, msgOut.Message.Content)
	}
	return nil
}

func processAgentOutput(writer server.ResponseEventWriter, output *adk.AgentOutput) error {
	if output == nil || output.MessageOutput == nil {
		return nil
	}

	msgOut := output.MessageOutput
	if msgOut.Role == schema.Tool {
		if err := writeToContent(writer, toolResultHeader(msgOut.ToolName)); err != nil {
			return err
		}
		if err := streamMessageToWriter(writer, msgOut); err != nil {
			return err
		}
		return writeToContent(writer, toolResultFooter)
	}

	return streamMessageToWriter(writer, msgOut)
}

func writeToContent(writer server.ResponseEventWriter, content string) error {
	return writer.Write(
		models.ResponseEvent{
			TaskStatusUpdateEventContent: &models.TaskStatusUpdateEventContent{
				Status: models.TaskStatus{
					State:   models.TaskStateWorking,
					Message: newAgentTextMessage(content),
				},
				Final: false,
			},
		})
}
