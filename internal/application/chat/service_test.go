package chat

import (
	"context"
	"strings"
	"testing"
)

func TestChatRequiresInitializedDefaultService(t *testing.T) {
	original := defaultService
	defaultService = nil
	t.Cleanup(func() { defaultService = original })

	_, err := Chat(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "chat service is not initialized") {
		t.Fatalf("Chat() error = %v, want service initialization error", err)
	}
}

func TestServiceWithKnowledgeContextWithoutKnowledgeClient(t *testing.T) {
	service := &Service{}
	input, err := service.withKnowledgeContext(context.Background(), "hello")
	if err != nil {
		t.Fatalf("withKnowledgeContext() error = %v", err)
	}
	if input != "hello" {
		t.Fatalf("withKnowledgeContext() = %q, want original message", input)
	}
}

func TestServiceChatRequiresRuntime(t *testing.T) {
	service := &Service{}
	_, err := service.Chat(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "chat service is not initialized") {
		t.Fatalf("Service.Chat() error = %v, want service initialization error", err)
	}
}
