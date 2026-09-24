// Package a2a exposes the chat application through the official A2A SDK.
// Wire formats and task lifecycle handling belong to the SDK; campus identity
// and the context-to-session mapping stay inside this inbound adapter.
package a2a

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	event "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	session "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	auth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	protocol "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
)

const Prefix = "/a2a"
const maxEntries = 1000

type ChatService interface {
	CreateSession(context.Context, string) (session.Session, error)
	StreamSession(context.Context, string, string, func(event.Event), string) (string, error)
	ValidateModelTier(string) error
}

// Authenticator must return a verified, stable user identity and a context with
// the credentials needed by ChatService. Client-provided user IDs are not trusted.
type Authenticator func(context.Context, string) (context.Context, string, error)
type Config struct {
	Authenticate Authenticator
}
type principalKey struct{}

func principal(ctx context.Context) (string, error) {
	id, _ := ctx.Value(principalKey{}).(string)
	if id == "" {
		return "", errors.New("authenticated user required")
	}
	return id, nil
}

func campusAuth(ctx context.Context, header string) (context.Context, string, error) {
	token, err := auth.ExtractBearerToken(header)
	if err != nil {
		return ctx, "", err
	}
	ctx = auth.WithAccessToken(ctx, token)
	id, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return ctx, "", errors.New("invalid campus access token")
	}
	return ctx, id, nil
}

func New(service ChatService, cfg Config) (http.Handler, error) {
	if service == nil {
		return nil, errors.New("A2A chat service is required")
	}
	if cfg.Authenticate == nil {
		cfg.Authenticate = campusAuth
	}
	ex := &executor{service: service, sessions: make(map[string]string), running: make(map[protocol.TaskID]context.CancelFunc)}
	store := &boundedStore{InMemory: taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: principal})}
	handler := a2asrv.NewHandler(ex, a2asrv.WithTaskStore(store), a2asrv.WithAgentInactivityTimeout(2*time.Minute))
	card := &protocol.AgentCard{
		Name: "TongjiStudent2.0", Version: "1.0.0", Description: "同济同学：校园问答、信息检索与多轮任务协助。个人校园数据使用当前用户授权。",
		Capabilities:      protocol.AgentCapabilities{Streaming: true},
		DefaultInputModes: []string{"text/plain"}, DefaultOutputModes: []string{"text/plain"},
		SecuritySchemes:      protocol.NamedSecuritySchemes{"campusBearer": protocol.HTTPAuthSecurityScheme{Scheme: "Bearer", Description: "Per-user Tongji OAuth access token"}},
		SecurityRequirements: protocol.SecurityRequirementsOptions{{"campusBearer": protocol.SecuritySchemeScopes{}}},
		Skills:               []protocol.AgentSkill{{ID: "tongji-student", Name: "同济校园助手", Description: "复用 TongjiStudent Agent 的检索、工具调用和会话能力", Tags: []string{"campus", "tongji"}, Examples: []string{"介绍一下你能做什么"}}},
	}
	mux := http.NewServeMux()
	publicCard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base, err := requestBaseURL(r)
		if err != nil {
			http.Error(w, "invalid request host", http.StatusBadRequest)
			return
		}
		// Never mutate a shared card: simultaneous requests can use different hosts.
		requestCard := *card
		requestCard.SupportedInterfaces = []*protocol.AgentInterface{
			protocol.NewAgentInterface(base+"/v1", protocol.TransportProtocolJSONRPC),
			{URL: base + "/v0.3", ProtocolBinding: protocol.TransportProtocolJSONRPC, ProtocolVersion: "0.3"},
		}
		w.Header().Set("Cache-Control", "no-store")
		a2asrv.NewAgentCardHandler(a2av0.NewStaticAgentCardProducer(&requestCard)).ServeHTTP(w, r)
	})
	mux.Handle(Prefix+"/.well-known/agent-card.json", publicCard)
	mux.Handle(Prefix+"/.well-known/agent.json", publicCard)
	mux.Handle(Prefix+"/v1", protect(cfg.Authenticate, a2asrv.NewJSONRPCHandler(handler)))
	mux.Handle(Prefix+"/v0.3", protect(cfg.Authenticate, a2av0.NewJSONRPCHandler(handler)))
	return mux, nil
}

// requestBaseURL uses the current request authority, including its port. Hertz
// supplies the scheme on URL; net/http supplies TLS for direct HTTPS requests.
// Forwarded headers are not trusted without a configured proxy trust boundary.
func requestBaseURL(r *http.Request) (string, error) {
	scheme := "http"
	if r.TLS != nil || r.URL.Scheme == "https" {
		scheme = "https"
	}
	u, err := url.Parse(scheme + "://" + r.Host)
	if err != nil || r.Host == "" || u.Host != r.Host || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(r.Host, " \t\r\n\\/?#") {
		return "", errors.New("invalid request authority")
	}
	u.Path = Prefix
	return u.String(), nil
}

func protect(authenticate Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST required", 405)
			return
		}
		ctx, owner, err := authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil || strings.TrimSpace(owner) == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="TongjiStudent A2A"`)
			http.Error(w, "valid campus access token required", 401)
			return
		}
		ctx = context.WithValue(ctx, principalKey{}, owner)
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Bounded single-process task storage. A deployment needing retention across
// restarts or multiple replicas should supply durable A2A task/context storage.
type boundedStore struct {
	*taskstore.InMemory
	mu    sync.Mutex
	count int
}

func (s *boundedStore) Create(ctx context.Context, task *protocol.Task) (taskstore.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count >= maxEntries {
		return 0, errors.New("A2A task capacity reached")
	}
	version, err := s.InMemory.Create(ctx, task)
	if err == nil {
		s.count++
	}
	return version, err
}
