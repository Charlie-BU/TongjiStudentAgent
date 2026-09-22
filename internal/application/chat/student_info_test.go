package chat

import (
	"context"
	"fmt"
	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadFormattedStudentInfo(t *testing.T) {
	setupStudentIdentity(t)
	Convey("通过复用的同济客户端加载学生信息", t, func() {
		calls := 0
		var tokens []string
		status := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			tokens = append(tokens, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			fmt.Fprint(w, `{"code":"A00000","msg":"ok","data":[{"currentGrade":2023,"faculty":"测试学院","leaveSchool":"校内在读","name":"测试同学","trainingLevel":"本科"}]}`)
		}))
		defer server.Close()
		client, err := tongjiapi.New(tongjiapi.Config{ClientID: "test", ClientSecret: "test", RedirectURI: "https://example.test/callback", StateSecret: "test", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, APIBaseURL: server.URL})
		So(err, ShouldBeNil)
		service := &Service{tongjiClient: client}
		info, err := service.loadFormattedStudentInfo(context.Background())
		So(err, ShouldBeNil)
		So(info, ShouldBeBlank)
		So(calls, ShouldEqual, 0)

		for _, token := range []string{"test-token-a", "test-token-b"} {
			info, err = service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), token))
			So(err, ShouldBeNil)
			So(info, ShouldEqual, "当前年级：2023\n学院：测试学院\n在校状态：校内在读\n姓名：测试同学\n培养层次：本科")
			So(service.tongjiClient, ShouldEqual, client)
		}
		So(tokens, ShouldResemble, []string{"Bearer service-token", "Bearer service-token"})
		status = http.StatusServiceUnavailable
		_, err = service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-token"))
		So(err, ShouldNotBeNil)
	})
	Convey("携带 token 时不能因客户端缺失静默跳过", t, func() {
		service := &Service{}
		_, err := service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-token"))
		So(err, ShouldNotBeNil)
		info, err := service.loadFormattedStudentInfo(context.Background())
		So(err, ShouldBeNil)
		So(info, ShouldBeBlank)
	})
}

// Exercise the full turn so optional context failures cannot prevent persistence or completion.
func TestStreamSessionOptionalStudentInfo(t *testing.T) {
	for _, tc := range []struct {
		name, token, body string
		status            int
		wantInfo          bool
	}{
		{"anonymous", "", "", 200, false},
		{"invalid token", "invalid", `{}`, 401, false},
		{"non student", "teacher", `{"code":"A00500","msg":"学号不能为空"}`, 200, false},
		{"empty profile", "empty", `{"code":"A00000","data":[]}`, 200, false},
		{"upstream unavailable", "unavailable", `{}`, 503, false},
		{"student", "student", `{"code":"A00000","data":[{"name":"测试同学"}]}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupStudentIdentity(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := tongjiapi.New(tongjiapi.Config{ClientID: "test", ClientSecret: "test", RedirectURI: "https://example.test/callback", StateSecret: "test", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, APIBaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			operations := []string{}
			store := &recordingEphemeralStore{operations: &operations}
			runner := &studentContextRuntime{recordingSessionRuntime: recordingSessionRuntime{operations: &operations, response: "回答"}}
			service := &Service{tongjiClient: client, runtimes: map[string]modelRuntime{"lite": {runtime: runner}}, ephemeralSessionStore: store, durableSessionStore: &studentDurableStore{store}, taskPlanRepository: &recordingTaskPlanRepository{}, turnLocker: noOpTurnLocker{}}
			events := []agentevent.Event{}
			ctx := platformauth.WithAccessToken(context.Background(), tc.token)
			response, err := service.StreamSession(ctx, "anon-test", "问题", func(e agentevent.Event) { events = append(events, e) }, "lite")
			if err != nil || response != "回答" {
				t.Fatalf("response=%q err=%v", response, err)
			}
			if len(store.appended) != 2 {
				t.Fatalf("expected user and assistant persistence: %#v", store.appended)
			}
			if (strings.Contains(runner.studentInfo, "测试同学")) != tc.wantInfo {
				t.Fatalf("unexpected student context %q", runner.studentInfo)
			}
			if !tc.wantInfo && runner.studentInfo != "" {
				t.Fatalf("unexpected context %q", runner.studentInfo)
			}
			for _, e := range events {
				if e.Type == agentevent.RunFailed {
					t.Fatalf("unexpected failure: %#v", e)
				}
			}
			if events[len(events)-1].Type != agentevent.RunCompleted {
				t.Fatal("turn did not complete")
			}
			wantCalls := 1
			if tc.token == "" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("profile requests=%d want=%d", calls, wantCalls)
			}
		})
	}
}

type studentContextRuntime struct {
	recordingSessionRuntime
	studentInfo string
}

func (r *studentContextRuntime) StreamWithHistoryAndMessages(ctx context.Context, query, info string, history []agenticsession.Message, emit func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error) {
	r.studentInfo = info
	return r.recordingSessionRuntime.StreamWithHistoryAndMessages(ctx, query, info, history, emit, record)
}

// Mock only the per-run identity boundary; profile failures remain independent.
func setupStudentIdentity(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			fmt.Fprint(w, `{"access_token":"service-token","expires_in":7200}`)
			return
		}
		if r.Header.Get("Authorization") == "Bearer service-token" {
			fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"00001","name":"李建中","userTypeName":"教职工"}]}}`)
			return
		}
		fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"student-1"}]}}`)
	}))
	t.Cleanup(server.Close)
	for key, value := range map[string]string{"TONGJI_LOGIN_CLIENT_ID": "login", "TONGJI_LOGIN_CLIENT_SECRET": "secret", "TONGJI_OPEN_PLATFORM_REDIRECT_URI": "https://example.test", "TONGJI_OPEN_PLATFORM_STATE_SECRET": "state", "TONGJI_OPEN_PLATFORM_API_BASE_URL": server.URL} {
		t.Setenv(key, value)
	}
	t.Setenv("TONGJI_MCP_CLIENT_ID", "service")
	t.Setenv("TONGJI_MCP_CLIENT_SECRET", "secret")
	t.Setenv("TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT", server.URL+"/token")
}

type studentDurableStore struct{ *recordingEphemeralStore }

func (s *studentDurableStore) Create(ctx context.Context, owner string) (agenticsession.Session, error) {
	return s.recordingEphemeralStore.Create(ctx)
}
func (s *studentDurableStore) Get(ctx context.Context, id, owner string) (agenticsession.Session, error) {
	return s.recordingEphemeralStore.Get(ctx, id)
}
func (s *studentDurableStore) Append(ctx context.Context, id, owner string, m agenticsession.NewMessage) (agenticsession.AppendResult, error) {
	return s.recordingEphemeralStore.Append(ctx, id, m)
}
func (s *studentDurableStore) ListMessages(ctx context.Context, id, owner string, limit int) ([]agenticsession.Message, error) {
	return s.recordingEphemeralStore.ListMessages(ctx, id, limit)
}
