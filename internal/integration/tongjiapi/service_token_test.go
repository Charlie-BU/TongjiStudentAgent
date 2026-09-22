package tongjiapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceTokenCache(t *testing.T) {
	var issued, checked atomic.Int32
	invalid, wrongIdentity, tokenFailure := false, false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			issued.Add(1)
			if tokenFailure {
				w.WriteHeader(503)
				return
			}
			r.ParseForm()
			if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("client_id") != "service" || r.PostForm.Get("client_secret") != "secret" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Error("invalid client credentials request")
			}
			fmt.Fprintf(w, `{"access_token":"service-%d","expires_in":7200}`, issued.Load())
			return
		}
		checked.Add(1)
		if invalid && r.Header.Get("Authorization") == "Bearer service-1" {
			w.WriteHeader(401)
			return
		}
		id := "00001"
		if wrongIdentity {
			id = "student"
		}
		fmt.Fprintf(w, `{"code":"A00000","data":{"list":[{"userId":%q,"name":"李建中","userTypeName":"教职工"}]}}`, id)
	}))
	defer server.Close()
	client := &Client{config: Config{ClientID: "service", ClientSecret: "secret", TokenEndpoint: server.URL + "/token", APIBaseURL: server.URL}, httpClient: server.Client()}
	cache := &ServiceTokenCache{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := cache.AccessToken(context.Background(), client)
			if err != nil || token != "service-1" {
				t.Errorf("token=%q err=%v", token, err)
			}
		}()
	}
	wg.Wait()
	if issued.Load() != 1 || checked.Load() != 8 {
		t.Fatalf("issued=%d checked=%d", issued.Load(), checked.Load())
	}
	invalid = true
	token, err := cache.AccessToken(context.Background(), client)
	if err != nil || token != "service-2" || issued.Load() != 2 {
		t.Fatalf("refresh: token=%q err=%v", token, err)
	}
	cache.expiresAt = time.Now().Add(-time.Second)
	token, err = cache.AccessToken(context.Background(), client)
	if err != nil || token != "service-3" {
		t.Fatalf("expiry: token=%q err=%v", token, err)
	}
	wrongIdentity = true
	if token, err = cache.AccessToken(context.Background(), client); err == nil || token != "" || cache.token != "" {
		t.Fatal("unexpected identity must clear cache")
	}
	tokenFailure = true
	if token, err = cache.AccessToken(context.Background(), client); err == nil || token != "" {
		t.Fatal("must fail closed")
	}
}

// A slow identity check must not block a different cached-token caller.
func TestServiceTokenCacheConcurrentValidation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"00001","name":"李建中","userTypeName":"教职工"}]}}`)
	}))
	defer server.Close()
	defer close(release)
	client := &Client{config: Config{APIBaseURL: server.URL}, httpClient: server.Client()}
	cache := &ServiceTokenCache{token: "cached", expiresAt: time.Now().Add(time.Hour)}
	first := make(chan error, 1)
	go func() { _, err := cache.AccessToken(context.Background(), client); first <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	token, err := cache.AccessToken(ctx, client)
	if err != nil || token != "cached" {
		t.Fatalf("independent validation: token=%q err=%v", token, err)
	}
}

func TestServiceTokenCacheRefreshWaitCancellation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client := &Client{config: Config{TokenEndpoint: server.URL}, httpClient: server.Client()}
	cache := &ServiceTokenCache{}
	ownerCtx, cancelOwner := context.WithCancel(context.Background())
	defer cancelOwner()
	owner := make(chan error, 1)
	go func() { _, err := cache.AccessToken(ownerCtx, client); owner <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	waiter := make(chan error, 1)
	go func() { _, err := cache.AccessToken(ctx, client); waiter <- err }()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiter err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter blocked behind refresh")
	}
	cancelOwner()
	if err := <-owner; !errors.Is(err, context.Canceled) {
		t.Fatalf("owner err=%v", err)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.refreshing != nil {
		t.Fatal("cancelled refresh left waiters blocked")
	}
}

func TestServiceTokenCacheCancelledValidationPreservesCache(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer server.Close()
	client := &Client{config: Config{APIBaseURL: server.URL}, httpClient: server.Client()}
	cache := &ServiceTokenCache{token: "cached", expiresAt: time.Now().Add(time.Hour)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := cache.AccessToken(ctx, client); done <- err }()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if cache.token != "cached" {
		t.Fatal("cancelled caller cleared shared token")
	}
}

func TestServiceTokenCacheStaleValidationDoesNotRefreshAgain(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var oldChecks, issued atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			issued.Add(1)
			fmt.Fprint(w, `{"access_token":"new","expires_in":7200}`)
			return
		}
		if r.Header.Get("Authorization") == "Bearer old" {
			if oldChecks.Add(1) == 1 {
				close(entered)
				<-release
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"userId":"00001","name":"李建中","userTypeName":"教职工"}]}}`)
	}))
	defer server.Close()
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	client := &Client{config: Config{TokenEndpoint: server.URL + "/token", APIBaseURL: server.URL}, httpClient: server.Client()}
	cache := &ServiceTokenCache{token: "old", expiresAt: time.Now().Add(time.Hour)}
	first := make(chan error, 1)
	go func() {
		token, err := cache.AccessToken(context.Background(), client)
		if err == nil && token != "new" {
			err = fmt.Errorf("token=%q", token)
		}
		first <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if token, err := cache.AccessToken(ctx, client); err != nil || token != "new" {
		t.Fatalf("refresh: token=%q err=%v", token, err)
	}
	unblock()
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if issued.Load() != 1 {
		t.Fatalf("stale validation caused %d refreshes", issued.Load())
	}
}
