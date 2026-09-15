package modelprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewFromEnv(t *testing.T) {
	Convey("供应商选择和缺失配置", t, func() {
		t.Setenv("ARK_API_KEY", "ark-test")
		t.Setenv("ARK_BASE_URL", "https://ark.example.test/api/v3")
		t.Setenv("OPENROUTER_API_KEY", "")
		for _, provider := range []string{"", "ark"} {
			t.Setenv("MODEL_PROVIDER", provider)
			m, err := NewFromEnv(context.Background(), "test-model", "lite")
			So(err, ShouldBeNil)
			So(m, ShouldNotBeNil)
		}
		t.Setenv("MODEL_PROVIDER", "openrouter")
		_, err := NewFromEnv(context.Background(), "vendor/test", "pro")
		So(err, ShouldNotBeNil)
		t.Setenv("MODEL_PROVIDER", "invalid")
		_, err = NewFromEnv(context.Background(), "test", "max")
		So(err, ShouldNotBeNil)
	})
}

func TestTierReasoningEffort(t *testing.T) {
	Convey("同一模型 ID 按档位设置推理强度，两个供应商一致", t, func() {
		for _, provider := range []string{"ark", "openrouter"} {
			t.Setenv("MODEL_PROVIDER", provider)
			var request map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&request)
				response := `{"id":"test-response","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`
				if request["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":%s}\n\n", response)
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, response)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("ARK_API_KEY", "test")
			t.Setenv("ARK_BASE_URL", server.URL)
			t.Setenv("OPENROUTER_API_KEY", "test")
			t.Setenv("OPENROUTER_BASE_URL", server.URL)
			for tier, effort := range map[string]string{"lite": "low", "pro": "medium", "max": "high"} {
				m, err := NewFromEnv(context.Background(), "same-model", tier)
				So(err, ShouldBeNil)
				_, err = m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
				So(err, ShouldBeNil)
				So(request["reasoning"].(map[string]any)["effort"], ShouldEqual, effort)
			}
			_, err := NewFromEnv(context.Background(), "same-model", "unknown")
			So(err, ShouldNotBeNil)
			server.Close()
		}
	})
}
