package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/runtime"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/modelprovider"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

// protocolTierStore 在离线测试中恢复每轮持久化的协议历史。
type protocolTierStore struct{ tierTestStore }

func (s *protocolTierStore) ListMessages(context.Context, string, int) ([]agenticsession.Message, error) {
	result := make([]agenticsession.Message, 0, len(s.messages))
	for i, m := range s.messages {
		result = append(result, agenticsession.Message{Sequence: int64(i + 1), Role: m.Role, Content: m.Content, ModelID: m.ModelID, ModelTier: m.ModelTier, ProtocolData: m.ProtocolData})
	}
	return result, nil
}

func TestOpenRouterTierIntegration(t *testing.T) {
	Convey("三档经过真实 Runtime 调用 Responses 并隔离模型元数据", t, func() {
		var requests []map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			requests = append(requests, req)
			modelID := req["model"].(string)
			data, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "id": "resp-test", "output": []any{
				map[string]any{"type": "reasoning", "encrypted_content": "private-" + modelID, "summary": []any{}},
				map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "answer-" + modelID}}},
			}}})
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\n", data)
		}))
		defer server.Close()
		t.Setenv("MODEL_PROVIDER", "openrouter")
		t.Setenv("OPENROUTER_API_KEY", "test-key")
		t.Setenv("OPENROUTER_BASE_URL", server.URL)
		t.Setenv("ARK_API_KEY", "")
		for _, tier := range []string{"LITE", "PRO", "MAX"} {
			t.Setenv(tier+"_MODEL", "vendor/"+tier)
		}
		fixture := newInitializationFixture("")
		fixture.deps.model = modelprovider.NewFromEnv
		service, err := fixture.deps.initialize(context.Background())
		So(err, ShouldBeNil)
		defer service.Close()
		store := &protocolTierStore{}
		service.ephemeralSessionStore = store
		service.turnLocker = noOpTurnLocker{}
		service.taskPlanRepository = &recordingTaskPlanRepository{}
		for _, tier := range []string{"lite", "pro", "max"} {
			selected, err := service.resolveModelTier(tier)
			So(err, ShouldBeNil)
			m, err := modelprovider.NewFromEnv(context.Background(), selected.modelID, tier)
			So(err, ShouldBeNil)
			rt, err := runtime.New(context.Background(), runtime.Config{Name: "test", Instruction: "固定规则", ChatModel: m})
			So(err, ShouldBeNil)
			service.runtimes[tier] = modelRuntime{runtime: rt, modelID: selected.modelID}
		}
		for _, tier := range []string{"lite", "lite", "pro", "max", "lite"} {
			selected, _ := service.resolveModelTier(tier)
			output, err := service.StreamSession(context.Background(), "anon-test", "hello", nil, tier)
			So(err, ShouldBeNil)
			So(output, ShouldEqual, "answer-"+selected.modelID)
		}
		So(requests, ShouldHaveLength, 5)
		for i, r := range requests {
			So(r["reasoning"].(map[string]any)["effort"], ShouldEqual, []string{"low", "low", "medium", "high", "low"}[i])
			So(r["session_id"], ShouldEqual, "anon-test")
			So(r["store"], ShouldEqual, false)
			So(r, ShouldNotContainKey, "previous_response_id")
		}
		same, _ := json.Marshal(requests[1]["input"])
		So(string(same), ShouldContainSubstring, "private-vendor/LITE")
		for _, i := range []int{2, 3, 4} {
			data, _ := json.Marshal(requests[i]["input"])
			So(string(data), ShouldNotContainSubstring, "private-vendor/")
			So(string(data), ShouldContainSubstring, "answer-vendor/LITE")
		}
		for _, m := range store.messages {
			if m.Role == agenticsession.MessageRoleAssistant {
				So(m.ProtocolData, ShouldNotBeBlank)
			}
		}
		t.Setenv("PRO_MODEL", "")
		t.Setenv("MAX_MODEL", "")
		f := newInitializationFixture("")
		f.deps.model = modelprovider.NewFromEnv
		liteOnly, err := f.deps.initialize(context.Background())
		So(err, ShouldBeNil)
		defer liteOnly.Close()
		_, err = liteOnly.resolveModelTier("pro")
		So(err, ShouldNotBeNil)
	})
}

func TestProtocolMetadataNotExposed(t *testing.T) {
	Convey("协议字段只用于服务端历史恢复", t, func() {
		m := schema.AssistantMessage("hello", nil)
		m.Extra = map[string]any{"model-protocol-data": "opaque"}
		stored, err := agenticsession.NewMessageFromSchema(m)
		So(err, ShouldBeNil)
		So(stored.ProtocolData, ShouldEqual, "opaque")
		data, err := json.Marshal(agenticsession.Message{Content: stored.Content, ProtocolData: stored.ProtocolData})
		So(err, ShouldBeNil)
		So(string(data), ShouldNotContainSubstring, "opaque")
	})
}
