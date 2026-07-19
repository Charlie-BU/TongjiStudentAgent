package knowledge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip 实现测试 HTTP 客户端的请求处理。
func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestFormatContext(t *testing.T) {
	Convey("格式化知识库上下文", t, func() {
		response := &SearchKnowledgeRes{Data: &SearchKnowledgeData{ResultList: []SearchResult{
			{ChunkTitle: "校车", Content: "班车在工作日运行。"},
			{OriginalQuestion: "何时选课？", Content: "请关注教务处通知。"},
		}}}

		Convey("包含普通切片和相似问题", func() {
			context := FormatContext(response)
			want := "[1] 校车\n班车在工作日运行。\n---\n相似问题：何时选课？\n参考答案：请关注教务处通知。\n"

			Convey("应保留资料内容和固定分隔格式", func() {
				So(context, ShouldEqual, want)
			})
		})
	})
}

func TestFormatContextEmptyResponse(t *testing.T) {
	Convey("格式化空知识库响应", t, func() {
		Convey("响应为 nil", func() {
			context := FormatContext(nil)

			Convey("应返回空上下文", func() {
				So(context, ShouldBeBlank)
			})
		})
	})
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

	Convey("从环境变量创建知识库客户端", t, func() {
		for _, test := range tests {
			test := test
			Convey(test.name, func() {
				for _, key := range keys {
					t.Setenv(key, "")
				}
				for key, value := range test.env {
					t.Setenv(key, value)
				}

				client, err := NewFromEnv()
				if test.wantErr != "" {
					Convey("应拒绝无效配置", func() {
						So(client, ShouldBeNil)
						So(err, ShouldNotBeNil)
						So(err.Error(), ShouldContainSubstring, test.wantErr)
					})
					return
				}

				if test.wantNil {
					Convey("开关关闭时不创建客户端", func() {
						So(err, ShouldBeNil)
						So(client, ShouldBeNil)
					})
					return
				}

				Convey("应创建并标准化客户端配置", func() {
					So(err, ShouldBeNil)
					So(client, ShouldNotBeNil)
					So(client.ak, ShouldEqual, "ak")
					So(client.sk, ShouldEqual, "sk")
					So(client.collection, ShouldEqual, "campus")
					So(client.project, ShouldEqual, "project")
					So(client.resourceID, ShouldEqual, "resource")
					So(client.limit, ShouldEqual, test.wantLimit)
					So(client.domain, ShouldEqual, test.wantDomain)
				})
			})
		}
	})
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

	Convey("检索知识库", t, func() {
		for _, test := range tests {
			test := test
			Convey(test.name, func() {
				client := &Client{ak: "test-ak", sk: "test-sk", domain: "knowledge.example.com", collection: "campus", project: "default", resourceID: "resource", limit: 3}
				client.httpClient = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
					So(request.Method, ShouldEqual, http.MethodPost)
					So(request.URL.Scheme, ShouldEqual, "https")
					So(request.URL.Host, ShouldEqual, client.domain)
					So(request.URL.Path, ShouldEqual, searchPath)
					So(request.Header.Get("Accept"), ShouldEqual, "application/json")
					So(request.Header.Get("Content-Type"), ShouldEqual, "application/json")
					So(request.Header.Get("Host"), ShouldEqual, client.domain)
					So(request.Header.Get("Authorization"), ShouldNotBeBlank)
					So(request.Header.Get("X-Date"), ShouldNotBeBlank)
					So(request.Header.Get("X-Content-Sha256"), ShouldNotBeBlank)
					body, err := io.ReadAll(request.Body)
					So(err, ShouldBeNil)
					want := `{"name":"campus","project":"default","resource_id":"resource","query":"where is the library?","limit":3,"dense_weight":0.5,"pre_processing":{"need_instruction":true},"post_processing":{"retrieve_count":3,"chunk_group":true}}`
					So(string(body), ShouldEqual, want)
					if test.roundTripErr != nil {
						return nil, test.roundTripErr
					}
					return &http.Response{StatusCode: test.responseCode, Body: io.NopCloser(strings.NewReader(test.responseBody)), Header: make(http.Header)}, nil
				})}

				result, err := client.Search(context.Background(), "where is the library?")
				if test.wantErr != "" {
					Convey("上游失败时应返回可定位错误", func() {
						So(err, ShouldNotBeNil)
						So(err.Error(), ShouldContainSubstring, test.wantErr)
					})
					return
				}

				Convey("上游成功时应解析检索结果", func() {
					So(err, ShouldBeNil)
					So(result, ShouldNotBeNil)
					So(result.Data, ShouldNotBeNil)
					So(result.Data.ResultList, ShouldHaveLength, 1)
					So(result.Data.ResultList[0].Content, ShouldEqual, "answer")
				})
			})
		}
	})
}
