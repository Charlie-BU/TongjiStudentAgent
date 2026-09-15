package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	sessioncontext "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/context"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
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
	Convey("显式档位并发路由及非法档位拒绝", t, func() {
		store := &tierTestStore{}
		s := &Service{runtimes: map[string]modelRuntime{}, ephemeralSessionStore: store, turnLocker: noOpTurnLocker{}, taskPlanRepository: &recordingTaskPlanRepository{}}
		for _, tier := range []string{"lite", "pro", "max"} {
			s.runtimes[tier] = modelRuntime{runtime: tierTestRuntime(tier), modelID: "model-" + tier}
		}
		var wg sync.WaitGroup
		results := make(chan bool, 30)
		for i := 0; i < 10; i++ {
			for _, tier := range []string{"lite", "pro", "max"} {
				wg.Add(1)
				go func(tier string) {
					defer wg.Done()
					out, err := s.StreamSession(context.Background(), "anon-"+tier, "hello", nil, tier)
					results <- (err == nil && out == tier)
				}(tier)
			}
		}
		wg.Wait()
		close(results)
		for correct := range results {
			So(correct, ShouldBeTrue)
		}
		So(store.messages, ShouldHaveLength, 60)
		for _, m := range store.messages {
			So(m.ModelID, ShouldEqual, "model-"+m.ModelTier)
		}
		before := len(store.messages)
		for _, tier := range []string{"", "PRO", "other"} {
			_, err := s.StreamSession(context.Background(), "anon", "hello", nil, tier)
			So(errors.Is(err, ErrInvalidModelTier), ShouldBeTrue)
		}
		delete(s.runtimes, "pro")
		_, err := s.StreamSession(context.Background(), "anon", "hello", nil, "pro")
		So(errors.Is(err, ErrModelTierUnavailable), ShouldBeTrue)
		So(store.messages, ShouldHaveLength, before)
	})
}

func TestHistoryModelCacheIsolation(t *testing.T) {
	Convey("模型切换清理响应缓存且不修改历史", t, func() {
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
			Convey(tc.name, func() {
				history := make([]agenticsession.Message, len(tc.models))
				for i, id := range tc.models {
					history[i] = agenticsession.Message{Sequence: int64(i + 1), Role: agenticsession.MessageRoleAssistant, Content: "answer", ModelID: id, ResponseID: "response", ResponseCacheExpiresAt: time.Now().Add(time.Hour).Unix()}
				}
				filtered := sanitizeHistoryForModel(history, tc.selected)
				messages, err := sessioncontext.NewContextAssembler().AssembleForTurn(context.Background(), sessioncontext.TurnInput{History: filtered, DynamicReminder: schema.UserMessage("reminder"), UserMessage: schema.UserMessage("query")})
				So(err, ShouldBeNil)
				count := 0
				for i, m := range filtered {
					So(m.ResponseID != "", ShouldEqual, tc.want[i])
					if tc.want[i] {
						count++
					}
					So(history[i].ResponseID, ShouldEqual, "response")
				}
				actual := 0
				for _, m := range messages {
					if m.Extra["ark-response-id"] != nil {
						actual++
					}
				}
				So(actual, ShouldEqual, count)
			})
		}
	})
}
