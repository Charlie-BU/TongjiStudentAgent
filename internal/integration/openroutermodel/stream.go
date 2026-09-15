package openroutermodel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	sdkstream "github.com/OpenRouterTeam/go-sdk/types/stream"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Stream 实时输出文本并在完成后交付完整工具调用和协议元数据。
func (m *chatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	payload, names, err := m.request(input, opts...)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithCancel(ctx) // 用于取消模型调用
	resp, err := m.send(requestCtx, payload)
	if err != nil {
		cancel()
		return nil, err
	}
	var closeOnce sync.Once
	closeUpstream := func() {
		closeOnce.Do(func() {
			cancel()
			_ = resp.Body.Close()
		})
	}
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		defer closeUpstream()
		// 响应体由本函数统一关闭，避免解析器与消费者退出时重复关闭。
		err := m.readEvents(struct{ io.Reader }{resp.Body}, names, func(msg *schema.Message) bool { return writer.Send(msg, nil) })
		if err != nil {
			writer.Send(nil, err)
		}
	}()
	return closeAwareStream(reader, closeUpstream), nil
}

// closeAwareStream 将消费者关闭传递给上游，即使源流正阻塞在 Recv。
// Eino 没有公开的 Close 回调：无缓冲请求流始终等待下一次 Recv 或 Close，
// 转换回调只读取真实消息，不向调用方暴露占位值。
func closeAwareStream(source *schema.StreamReader[*schema.Message], closeUpstream func()) *schema.StreamReader[*schema.Message] {
	requests, writer := schema.Pipe[struct{}](0)
	go func() {
		defer writer.Close()
		defer source.Close()
		defer closeUpstream()
		for !writer.Send(struct{}{}, nil) {
		}
	}()
	return schema.StreamReaderWithConvert(requests, func(struct{}) (*schema.Message, error) {
		return source.Recv()
	})
}

// readEvents 校验流完整性并避免完成事件重复输出文本。
func (m *chatModel) readEvents(body io.Reader, names map[string]string, send func(*schema.Message) bool) error {
	events := sdkstream.NewEventStream(context.Background(), body, func(raw []byte) (streamEvent, error) {
		var envelope streamEnvelope
		err := json.Unmarshal(raw, &envelope)
		return envelope.Data, err
	}, "[DONE]")
	defer events.Close()
	var text strings.Builder
	reasoning := map[int]*reasoningProgress{}
	completed := false
	dispatch := func(event streamEvent) error {
		switch event.Type {
		case "response.output_text.delta", "response.refusal.delta":
			text.WriteString(event.Delta)
			if send(schema.AssistantMessage(event.Delta, nil)) {
				return io.ErrClosedPipe
			}
		case "response.reasoning.delta", "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
			kind := "content"
			if event.Type == "response.reasoning_summary_text.delta" {
				kind = "summary"
			}
			if event.Delta == "" {
				break
			}
			progress := reasoning[event.OutputIndex]
			if progress == nil {
				progress = &reasoningProgress{Kind: kind}
				reasoning[event.OutputIndex] = progress
			}
			// 同一 item 只展示一种可见表示，避免上游同时返回正文和摘要时重复。
			if progress.Kind != kind {
				break
			}
			progress.Text += event.Delta
			if send(&schema.Message{Role: schema.Assistant, ReasoningContent: event.Delta}) {
				return io.ErrClosedPipe
			}
		case "response.completed":
			msg, err := m.message(event.Response, names)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(msg.Content, text.String()) {
				return fmt.Errorf("OpenRouter stream text differs from completed response")
			}
			msg.Content = strings.TrimPrefix(msg.Content, text.String())
			for index, raw := range event.Response.Output {
				var out item
				if err := json.Unmarshal(raw, &out); err != nil {
					return err
				}
				if out.Type != "reasoning" {
					continue
				}
				progress := reasoning[index]
				if progress == nil {
					msg.ReasoningContent += visibleReasoning(out, "summary")
					continue
				}
				final := visibleReasoning(out, progress.Kind)
				// 完成对象可能省略正文或仅提供另一个摘要；保留已经收到的可见增量。
				if strings.HasPrefix(final, progress.Text) {
					msg.ReasoningContent += strings.TrimPrefix(final, progress.Text)
				}
			}
			if send(msg) {
				return io.ErrClosedPipe
			}
			completed = true
		case "response.failed", "response.incomplete", "error":
			return fmt.Errorf("OpenRouter stream failed (%s)", event.Type)
		}
		return nil
	}
	for events.Next() {
		if err := dispatch(*events.Value()); err != nil {
			return err
		}
		if completed {
			return nil
		}
	}
	if err := events.Err(); err != nil {
		return fmt.Errorf("read OpenRouter stream: %w", err)
	}
	if !completed {
		return fmt.Errorf("OpenRouter stream ended before response.completed")
	}
	return nil
}
