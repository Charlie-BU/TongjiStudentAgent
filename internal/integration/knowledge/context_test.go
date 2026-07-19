package knowledge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip 实现测试 HTTP 客户端的请求处理。
func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestFormatContext(t *testing.T) {
	context := FormatContext(&SearchKnowledgeRes{Data: &SearchKnowledgeData{ResultList: []SearchResult{
		{ChunkTitle: "校车", Content: "班车在工作日运行。"},
		{OriginalQuestion: "何时选课？", Content: "请关注教务处通知。"},
	}}})

	want := "[1] 校车\n班车在工作日运行。\n---\n相似问题：何时选课？\n参考答案：请关注教务处通知。\n"
	if context != want {
		t.Fatalf("FormatContext() = %q, want %q", context, want)
	}
}

func TestFormatContextEmptyResponse(t *testing.T) {
	if context := FormatContext(nil); context != "" {
		t.Fatalf("FormatContext(nil) = %q, want empty string", context)
	}
}

func TestNewFromEnv(t *testing.T) {
	keys := []string{
		"ARK_KNOWLEDGE_ENABLED", "ARK_AK", "ARK_SK", "ARK_KNOWLEDGE_COLLECTION",
		"ARK_KNOWLEDGE_PROJECT", "ARK_KNOWLEDGE_RESOURCE_ID", "ARK_KNOWLEDGE_LIMIT",
		"ARK_KNOWLEDGE_DOMAIN",
	}
	tests := []struct {
		name       string
		env        map[string]string
		wantNil    bool
		wantErr    string
		wantLimit  int
		wantDomain string
	}{
		{name: "disabled", wantNil: true},
		{name: "invalid enabled value", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "enabled"}, wantErr: "parse ARK_KNOWLEDGE_ENABLED"},
		{name: "missing credentials", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "ARK_KNOWLEDGE_COLLECTION": "campus"}, wantErr: "ARK_AK and ARK_SK"},
		{name: "missing collection and resource ID", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "ARK_AK": "ak", "ARK_SK": "sk"}, wantErr: "ARK_KNOWLEDGE_COLLECTION or ARK_KNOWLEDGE_RESOURCE_ID"},
		{name: "invalid limit", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "ARK_AK": "ak", "ARK_SK": "sk", "ARK_KNOWLEDGE_COLLECTION": "campus", "ARK_KNOWLEDGE_LIMIT": "0"}, wantErr: "ARK_KNOWLEDGE_LIMIT"},
		{name: "custom configuration", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "ARK_AK": " ak ", "ARK_SK": " sk ", "ARK_KNOWLEDGE_COLLECTION": " campus ", "ARK_KNOWLEDGE_PROJECT": " project ", "ARK_KNOWLEDGE_RESOURCE_ID": " resource ", "ARK_KNOWLEDGE_LIMIT": "3", "ARK_KNOWLEDGE_DOMAIN": " knowledge.example.com "}, wantLimit: 3, wantDomain: "knowledge.example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "")
			}
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			client, err := NewFromEnv()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("NewFromEnv() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewFromEnv() error = %v", err)
			}
			if test.wantNil {
				if client != nil {
					t.Fatal("NewFromEnv() returned a client when disabled")
				}
				return
			}
			if client == nil {
				t.Fatal("NewFromEnv() returned nil client")
			}
			if client.ak != "ak" || client.sk != "sk" || client.collection != "campus" || client.project != "project" || client.resourceID != "resource" || client.limit != test.wantLimit || client.domain != test.wantDomain {
				t.Fatalf("NewFromEnv() client = %+v, unexpected configuration", client)
			}
		})
	}
}

func TestClientSearch(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		responseBody string
		roundTripErr error
		wantErr      string
	}{
		{name: "success", responseCode: http.StatusOK, responseBody: `{"code":0,"data":{"result_list":[{"content":"answer"}]}}`},
		{name: "HTTP error", responseCode: http.StatusBadGateway, responseBody: `gateway unavailable`, wantErr: "HTTP 502"},
		{name: "API error", responseCode: http.StatusOK, responseBody: `{"code":1001,"message":"invalid collection"}`, wantErr: "code 1001"},
		{name: "invalid JSON", responseCode: http.StatusOK, responseBody: `{`, wantErr: "unmarshal knowledge search response"},
		{name: "transport error", roundTripErr: errors.New("network unavailable"), wantErr: "send knowledge search request"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{ak: "test-ak", sk: "test-sk", domain: "knowledge.example.com", collection: "campus", project: "default", resourceID: "resource", limit: 3}
			client.httpClient = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.Scheme != "https" || request.URL.Host != client.domain || request.URL.Path != searchPath {
					t.Errorf("request = %s %s", request.Method, request.URL)
				}
				if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Host") != client.domain || request.Header.Get("Authorization") == "" || request.Header.Get("X-Date") == "" || request.Header.Get("X-Content-Sha256") == "" {
					t.Errorf("request headers = %#v", request.Header)
				}
				body, err := io.ReadAll(request.Body)
				if err != nil {
					t.Fatal(err)
				}
				want := `{"name":"campus","project":"default","resource_id":"resource","query":"where is the library?","limit":3,"dense_weight":0.5,"pre_processing":{"need_instruction":true},"post_processing":{"retrieve_count":3,"chunk_group":true}}`
				if string(body) != want {
					t.Errorf("request body = %s, want %s", body, want)
				}
				if test.roundTripErr != nil {
					return nil, test.roundTripErr
				}
				return &http.Response{StatusCode: test.responseCode, Body: io.NopCloser(strings.NewReader(test.responseBody)), Header: make(http.Header)}, nil
			})}

			result, err := client.Search(context.Background(), "where is the library?")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Search() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if result == nil || len(result.Data.ResultList) != 1 || result.Data.ResultList[0].Content != "answer" {
				t.Fatalf("Search() result = %#v", result)
			}
		})
	}
}
