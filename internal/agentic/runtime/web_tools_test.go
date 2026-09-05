package runtime

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

type webTransport func(*http.Request) (*http.Response, error)

func (f webTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// webModel 按脚本调用实际工具并检查回填结果。
type webModel struct {
	calls     int
	failed    bool
	sawSearch bool
	sawFetch  bool
}

func (m *webModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) { return m, nil }
func (m *webModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	for _, message := range messages {
		if message.Role == schema.Tool && message.ToolName == toolallowlist.WebSearchTool {
			m.sawSearch = strings.Contains(message.Content, `"status":"ok"`) || strings.Contains(message.Content, `"status":"rate_limited"`)
		}
		if message.Role == schema.Tool && message.ToolName == toolallowlist.URLFetchTool {
			m.sawFetch = strings.Contains(message.Content, "公开正文")
		}
	}
	if m.calls == 1 {
		return schema.AssistantMessage("", []schema.ToolCall{{ID: "search-1", Function: schema.FunctionCall{Name: toolallowlist.WebSearchTool, Arguments: `{"query":"公开公告","reason":"核验"}`}}}), nil
	}
	if !m.sawSearch {
		return nil, errors.New("missing search result")
	}
	if m.failed {
		return schema.AssistantMessage("搜索暂不可用，无法核验。", nil), nil
	}
	if m.calls == 2 {
		return schema.AssistantMessage("", []schema.ToolCall{{ID: "fetch-1", Function: schema.FunctionCall{Name: toolallowlist.URLFetchTool, Arguments: `{"url":"https://example.org/notice","reason":"核验正文"}`}}}), nil
	}
	if !m.sawFetch {
		return nil, errors.New("missing fetch result")
	}
	return schema.AssistantMessage("公开正文。[来源](https://example.org/notice)", nil), nil
}
func (m *webModel) Stream(ctx context.Context, messages []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.Generate(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

// TestRuntimeWebTools 验证真实工具到 HTTP 适配器的离线事件和记录链路。
func TestRuntimeWebTools(t *testing.T) {
	Convey("公开网页工具闭环", t, func() {
		for _, failed := range []bool{false, true} {
			calls := 0
			client, err := tavily.NewClient("fixture-key", time.Second, webTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				code := 200
				body := `{"results":[{"title":"公告","url":"https://example.org/notice","content":"摘要"}]}`
				if failed {
					code = 429
					body = "upstream secret"
				} else if r.URL.Path == "/extract" {
					body = `{"results":[{"url":"https://example.org/notice","raw_content":"公开正文"}],"failed_results":[]}`
				}
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			So(err, ShouldBeNil)
			m := &webModel{failed: failed}
			rt, err := New(context.Background(), Config{Name: "web-test", ChatModel: m, Tools: systemtools.Tools(systemtools.WithTavilyClient(client)), MaxIterations: 4})
			So(err, ShouldBeNil)
			var events []agentevent.Event
			var recorded []*schema.Message
			response, err := rt.StreamWithHistoryAndMessages(context.Background(), "核验公开公告", "", nil, func(e agentevent.Event) { events = append(events, e) }, func(_ context.Context, m *schema.Message) error { recorded = append(recorded, m); return nil })
			So(err, ShouldBeNil)
			want := 2
			if failed {
				want = 1
				So(response, ShouldContainSubstring, "无法核验")
			} else {
				So(response, ShouldContainSubstring, "https://example.org/notice")
			}
			So(calls, ShouldEqual, want)
			started, completed, toolMessages := 0, 0, 0
			for _, e := range events {
				if e.Type == agentevent.ToolCallStarted {
					started++
				}
				if e.Type == agentevent.ToolCallCompleted {
					completed++
				}
				So(e.Type, ShouldNotEqual, agentevent.ToolCallFailed)
			}
			for _, message := range recorded {
				if message.Role == schema.Tool {
					toolMessages++
				}
				So(message.Content, ShouldNotContainSubstring, "upstream secret")
				So(message.Content, ShouldNotContainSubstring, "fixture-key")
			}
			So(started, ShouldEqual, want)
			So(completed, ShouldEqual, want)
			So(toolMessages, ShouldEqual, want)
			So(recorded, ShouldHaveLength, want*2+1)
		}
	})
}
