package a2a

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

type fakeChat struct {
	mu       sync.Mutex
	created  int
	sessions []string
	release  <-chan struct{}
}

func (f *fakeChat) ValidateModelTier(string) error { return nil }
func (f *fakeChat) CreateSession(context.Context, string) (session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created++
	return session.Session{ID: fmt.Sprint(f.created)}, nil
}
func (f *fakeChat) StreamSession(ctx context.Context, id, query string, send func(event.Event), tier string) (string, error) {
	f.mu.Lock()
	f.sessions = append(f.sessions, id)
	f.mu.Unlock()
	if query == "fail" {
		return "", errors.New("upstream-secret")
	}
	if query == "fallback" {
		return "完整回答", nil
	}
	send(event.Event{Type: event.AssistantDelta, Data: event.AssistantDeltaData{Text: "第一段"}})
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	send(event.Event{Type: event.AssistantDelta, Data: event.AssistantDeltaData{Text: "第二段"}})
	return "第一段第二段", nil
}
func testHandler(t *testing.T, f *fakeChat) http.Handler {
	t.Helper()
	h, err := New(f, Config{Authenticate: func(ctx context.Context, h string) (context.Context, string, error) {
		if h != "Bearer alice" && h != "Bearer bob" {
			return ctx, "", errors.New("bad token")
		}
		return ctx, strings.TrimPrefix(h, "Bearer "), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func rpc(t *testing.T, url, owner, method string, params any) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "test", "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+owner)
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
func message(version, contextID string) map[string]any {
	role := "ROLE_USER"
	part := map[string]any{"text": "你好"}
	if version == "v0.3" {
		role = "user"
		part["kind"] = "text"
	}
	msg := map[string]any{"messageId": "m-1", "role": role, "parts": []any{part}}
	if contextID != "" {
		msg["contextId"] = contextID
	}
	return map[string]any{"message": msg}
}
func readJSON(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err = json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("%s: %s", err, b)
	}
	return obj
}
func TestWireStreamingAndIsolation(t *testing.T) {
	for _, version := range []string{"v1", "v0.3"} {
		t.Run(version, func(t *testing.T) {
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			f := &fakeChat{release: release}
			srv := httptest.NewServer(testHandler(t, f))
			defer srv.Close()
			method := "SendStreamingMessage"
			get := "GetTask"
			if version == "v0.3" {
				method = "message/stream"
				get = "tasks/get"
			}
			res := rpc(t, srv.URL+"/a2a/"+version, "alice", method, message(version, ""))
			defer res.Body.Close()
			if !strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("not SSE: %s", b)
			}
			scanner := bufio.NewScanner(res.Body)
			var wire strings.Builder
			first := false
			for scanner.Scan() {
				line := scanner.Text()
				wire.WriteString(line)
				wire.WriteByte('\n')
				if strings.Contains(line, "第一段") {
					first = true
					break
				}
			}
			if !first {
				t.Fatalf("no first delta before backend released: %s (%v)", wire.String(), scanner.Err())
			}
			unblock()
			for scanner.Scan() {
				wire.WriteString(scanner.Text())
				wire.WriteByte('\n')
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			raw := wire.String()
			if strings.Count(raw, "第一段") != 1 || strings.Count(raw, "第二段") != 1 || !strings.Contains(strings.ToLower(raw), "completed") || !strings.Contains(raw, `"lastChunk":true`) {
				t.Fatalf("bad stream: %s", raw)
			}
			// Extract task/context from the initial task event (1.0 wraps it in task).
			var taskID, contextID string
			for _, line := range strings.Split(raw, "\n") {
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				var obj map[string]any
				if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &obj) != nil {
					continue
				}
				result, _ := obj["result"].(map[string]any)
				if nested, ok := result["task"].(map[string]any); ok {
					result = nested
				}
				if id, ok := result["id"].(string); ok {
					taskID = id
					contextID, _ = result["contextId"].(string)
					break
				}
			}
			if taskID == "" || contextID == "" {
				t.Fatalf("missing task identity: %s", raw)
			}
			own := readJSON(t, rpc(t, srv.URL+"/a2a/"+version, "alice", get, map[string]any{"id": taskID}))
			if own["error"] != nil {
				t.Fatalf("owner cannot get task: %v", own)
			}
			foreign := readJSON(t, rpc(t, srv.URL+"/a2a/"+version, "bob", get, map[string]any{"id": taskID}))
			if foreign["error"] == nil {
				t.Fatalf("task leaked: %v", foreign)
			}
			next := rpc(t, srv.URL+"/a2a/"+version, "alice", method, message(version, contextID))
			nextBody, _ := io.ReadAll(next.Body)
			next.Body.Close()
			if strings.Contains(string(nextBody), `"error"`) {
				t.Fatalf("continuation failed: %s", nextBody)
			}
			foreignNext := rpc(t, srv.URL+"/a2a/"+version, "bob", method, message(version, contextID))
			foreignBody, _ := io.ReadAll(foreignNext.Body)
			foreignNext.Body.Close()
			if !strings.Contains(string(foreignBody), `"error"`) {
				t.Fatalf("foreign context accepted: %s", foreignBody)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.created != 1 || len(f.sessions) != 2 || f.sessions[0] != f.sessions[1] {
				t.Fatalf("wrong backend sessions: %+v", f.sessions)
			}
		})
	}
}
func TestDiscoveryAndAuthentication(t *testing.T) {
	srv := httptest.NewServer(testHandler(t, &fakeChat{}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/a2a/.well-known/agent-card.json")
	if err != nil {
		t.Fatal(err)
	}
	card := readJSON(t, res)
	raw, _ := json.Marshal(card)
	if !strings.Contains(string(raw), srv.URL+"/a2a/v1") || !strings.Contains(string(raw), "campusBearer") {
		t.Fatalf("wrong card: %s", raw)
	}
	denied := rpc(t, srv.URL+"/a2a/v1", "invalid", "SendMessage", message("v1", ""))
	denied.Body.Close()
	if denied.StatusCode != 401 {
		t.Fatalf("got %d", denied.StatusCode)
	}
}

func TestHertzStreamingAndCancel(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	f := &fakeChat{release: release}
	var transport network.Transporter
	h := server.New(server.WithHostPorts("127.0.0.1:0"), server.WithTransport(func(o *config.Options) network.Transporter { transport = standard.NewTransporter(o); return transport }))
	h.Any("/a2a/*path", adaptor.HertzHandler(testHandler(t, f)))
	done := make(chan error, 1)
	go func() { done <- h.Run() }()
	defer func() { h.Close(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for !h.IsRunning() {
		if time.Now().After(deadline) {
			t.Fatal("Hertz did not start")
		}
		time.Sleep(time.Millisecond)
	}
	base := "http://" + transport.(interface{ Listener() net.Listener }).Listener().Addr().String() + "/a2a/v0.3"
	res := rpc(t, base, "alice", "message/stream", message("v0.3", ""))
	defer res.Body.Close()
	scanner := bufio.NewScanner(res.Body)
	taskID := ""
	first := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			var obj struct {
				Result struct {
					ID string `json:"id"`
				}
			}
			json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &obj)
			if obj.Result.ID != "" {
				taskID = obj.Result.ID
			}
		}
		if strings.Contains(line, "第一段") {
			first = true
			break
		}
	}
	if !first || taskID == "" {
		t.Fatal("Hertz did not flush text before completion")
	}
	denied := readJSON(t, rpc(t, base, "bob", "tasks/cancel", map[string]any{"id": taskID}))
	if denied["error"] == nil {
		t.Fatal("foreign cancel allowed")
	}
	canceled := readJSON(t, rpc(t, base, "alice", "tasks/cancel", map[string]any{"id": taskID}))
	if canceled["error"] != nil {
		t.Fatalf("cancel failed: %v", canceled)
	}
	var tail strings.Builder
	for scanner.Scan() {
		tail.WriteString(scanner.Text())
	}
	if scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	if strings.Contains(tail.String(), "第二段") || strings.Contains(tail.String(), "completed") {
		t.Fatalf("execution continued after cancel: %s", tail.String())
	}
}

func TestRejectUnsupportedContent(t *testing.T) {
	f := &fakeChat{}
	srv := httptest.NewServer(testHandler(t, f))
	defer srv.Close()
	params := message("v0.3", "")
	params["message"].(map[string]any)["parts"] = []any{map[string]any{"kind": "data", "data": map[string]any{"secret": "test"}}}
	result := readJSON(t, rpc(t, srv.URL+"/a2a/v0.3", "alice", "message/send", params))
	if result["error"] == nil {
		t.Fatalf("unsupported data accepted: %v", result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.created != 0 {
		t.Fatal("backend invoked for unsupported input")
	}
}

func TestFailureAndNonStreamingFallback(t *testing.T) {
	srv := httptest.NewServer(testHandler(t, &fakeChat{}))
	defer srv.Close()
	for _, query := range []string{"fail", "fallback"} {
		params := message("v0.3", "")
		params["message"].(map[string]any)["parts"] = []any{map[string]any{"kind": "text", "text": query}}
		res := rpc(t, srv.URL+"/a2a/v0.3", "alice", "message/stream", params)
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		raw := string(body)
		if query == "fail" {
			if !strings.Contains(raw, "failed") || strings.Contains(raw, "completed") || strings.Contains(raw, "upstream-secret") {
				t.Fatalf("invalid failure projection: %s", raw)
			}
		} else if strings.Count(raw, "完整回答") != 1 || !strings.Contains(raw, "completed") {
			t.Fatalf("invalid fallback projection: %s", raw)
		}
	}
}

func TestRequestDerivedDiscovery(t *testing.T) {
	h := testHandler(t, &fakeChat{})
	for _, origin := range []string{"http://124.223.93.75:8080", "https://agent.example.com", "http://localhost:9090", "http://[::1]:8080"} {
		for _, path := range []string{"/a2a/.well-known/agent-card.json", "/a2a/.well-known/agent.json"} {
			t.Run(origin+path, func(t *testing.T) {
				t.Parallel()
				req := httptest.NewRequest("GET", origin+path, nil)
				req.Header.Set("Forwarded", "host=untrusted.example;proto=https")
				req.Header.Set("X-Forwarded-Host", "untrusted.example")
				req.Header.Set("X-Forwarded-Proto", "https")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				var card struct {
					URL        string `json:"url"`
					Interfaces []struct {
						URL string `json:"url"`
					} `json:"supportedInterfaces"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &card); err != nil {
					t.Fatal(err)
				}
				if w.Code != 200 || card.URL != origin+"/a2a/v0.3" || len(card.Interfaces) != 2 || card.Interfaces[0].URL != origin+"/a2a/v1" || card.Interfaces[1].URL != origin+"/a2a/v0.3" {
					t.Fatalf("wrong card: %s", w.Body.String())
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("request-specific card must not be cached")
				}
			})
		}
	}
	for _, host := range []string{"", "user:pass@example.com", "example.com/path", "example.com?x=y", "example.com#fragment", "bad host"} {
		req := httptest.NewRequest("GET", "http://example.com/a2a/.well-known/agent-card.json", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid host %q accepted: %d", host, w.Code)
		}
	}
}
