package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestNewRequiresChatModel(t *testing.T) {
	_, err := New(context.Background(), Config{})
	if err == nil || !strings.Contains(err.Error(), "chat model is required") {
		t.Fatalf("New() error = %v, want chat model validation error", err)
	}
}

func TestChatRequiresInitializedRuntime(t *testing.T) {
	var runtime *Runtime
	_, err := runtime.Chat(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "agent runtime is not initialized") {
		t.Fatalf("Chat() error = %v, want runtime initialization error", err)
	}
}
