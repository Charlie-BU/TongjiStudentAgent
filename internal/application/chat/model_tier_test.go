package chat

import (
	"context"
	"errors"
	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	sessioncontext "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/context"
	"github.com/cloudwego/eino/schema"
	"sync"
	"testing"
	"time"
)

type tierTestRuntime string

func (r tierTestRuntime) StreamWithHistoryAndMessages(ctx context.Context, query, student string, history []agenticsession.Message, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error) {
	selection := ctx.Value(modelSelectionKey{}).(modelSelection)
	if selection.tier != string(r) {
		return "", errors.New("wrong runtime")
	}
	return string(r), record(ctx, schema.AssistantMessage(string(r), nil))
}

type tierTestStore struct {
	mu       sync.Mutex
	messages []agenticsession.NewMessage
}

func (s *tierTestStore) Create(context.Context) (agenticsession.Session, error) {
	return agenticsession.Session{}, nil
}
func (s *tierTestStore) Get(context.Context, string) (agenticsession.Session, error) {
	return agenticsession.Session{ID: "anon-test", Persistence: agenticsession.PersistenceEphemeral}, nil
}
func (s *tierTestStore) ListMessages(context.Context, string, int) ([]agenticsession.Message, error) {
	return nil, nil
}
func (s *tierTestStore) Append(_ context.Context, _ string, m agenticsession.NewMessage) (agenticsession.AppendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
	return agenticsession.AppendResult{}, nil
}

func TestTierRoutingConcurrent(t *testing.T) {
	store := &tierTestStore{}
	s := &Service{runtimes: map[string]modelRuntime{}, ephemeralSessionStore: store, turnLocker: noOpTurnLocker{}, taskPlanRepository: &recordingTaskPlanRepository{}}
	for _, tier := range []string{"lite", "pro", "max"} {
		s.runtimes[tier] = modelRuntime{runtime: tierTestRuntime(tier), modelID: "model-" + tier}
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, tier := range []string{"lite", "pro", "max"} {
			wg.Add(1)
			go func(tier string) {
				defer wg.Done()
				out, err := s.StreamSession(context.Background(), "anon-"+tier, "hello", nil, tier)
				if err != nil || out != tier {
					t.Errorf("tier %s: %q %v", tier, out, err)
				}
			}(tier)
		}
	}
	wg.Wait()
	if len(store.messages) != 60 {
		t.Fatalf("messages: %d", len(store.messages))
	}
	for _, m := range store.messages {
		if m.ModelID != "model-"+m.ModelTier {
			t.Fatalf("metadata: %+v", m)
		}
	}
	before := len(store.messages)
	for _, tier := range []string{"", "PRO", "other"} {
		if _, err := s.StreamSession(context.Background(), "anon", "hello", nil, tier); !errors.Is(err, ErrInvalidModelTier) {
			t.Fatal(err)
		}
	}
	delete(s.runtimes, "pro")
	if _, err := s.StreamSession(context.Background(), "anon", "hello", nil, "pro"); !errors.Is(err, ErrModelTierUnavailable) {
		t.Fatal(err)
	}
	if len(store.messages) != before {
		t.Fatal("rejected request wrote messages")
	}
	out, err := s.StreamSession(context.Background(), "anon", "hello", nil)
	if err != nil || out != "lite" {
		t.Fatalf("default tier: %s %v", out, err)
	}
}

func TestHistoryModelCacheIsolation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		models   []string
		selected string
		want     []bool
	}{
		{"same", []string{"lite", "lite"}, "lite", []bool{true, true}},
		{"switch", []string{"lite", "lite"}, "pro", []bool{false, false}},
		{"switch back", []string{"lite", "pro"}, "lite", []bool{false, false}},
		{"resume new chain", []string{"lite", "pro", "lite"}, "lite", []bool{false, false, true}},
		{"legacy", []string{""}, "lite", []bool{false}},
		{"changed model", []string{"old-lite"}, "new-lite", []bool{false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := make([]agenticsession.Message, len(tc.models))
			for i, id := range tc.models {
				history[i] = agenticsession.Message{Sequence: int64(i + 1), Role: agenticsession.MessageRoleAssistant, Content: "answer", ModelID: id, ResponseID: "response", ResponseCacheExpiresAt: time.Now().Add(time.Hour).Unix()}
			}
			filtered := historyForModel(history, tc.selected)
			messages, err := sessioncontext.NewContextAssembler().AssembleForTurn(context.Background(), sessioncontext.TurnInput{History: filtered, DynamicReminder: schema.UserMessage("reminder"), UserMessage: schema.UserMessage("query")})
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for i, m := range filtered {
				if (m.ResponseID != "") != tc.want[i] {
					t.Fatalf("cache %d: %+v", i, m)
				}
				if tc.want[i] {
					count++
				}
				if history[i].ResponseID == "" {
					t.Fatal("mutated stored history")
				}
			}
			actual := 0
			for _, m := range messages {
				if m.Extra["ark-response-id"] != nil {
					actual++
				}
			}
			if actual != count {
				t.Fatalf("restored %d caches, want %d", actual, count)
			}
		})
	}
}
