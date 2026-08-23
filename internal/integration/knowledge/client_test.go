package knowledge

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewFromEnv(t *testing.T) {
	keys := []string{
		"ARK_KNOWLEDGE_ENABLED", "VOLC_API_KEY", "ARK_KNOWLEDGE_COLLECTION",
		"ARK_KNOWLEDGE_PROJECT", "ARK_KNOWLEDGE_RESOURCE_ID", "ARK_KNOWLEDGE_LIMIT",
		"ARK_KNOWLEDGE_DOMAIN", "ARK_KNOWLEDGE_REGION",
	}
	tests := []struct {
		name      string
		env       map[string]string
		wantNil   bool
		wantError string
		wantLimit int
	}{
		{name: "开关关闭", wantNil: true},
		{name: "开关值非法", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "enabled"}, wantError: "parse ARK_KNOWLEDGE_ENABLED"},
		{name: "缺少 API Key", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "ARK_KNOWLEDGE_COLLECTION": "campus"}, wantError: "VOLC_API_KEY"},
		{name: "缺少集合配置", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "VOLC_API_KEY": "test-key"}, wantError: "ARK_KNOWLEDGE_COLLECTION or ARK_KNOWLEDGE_RESOURCE_ID"},
		{name: "检索数量非法", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "VOLC_API_KEY": "test-key", "ARK_KNOWLEDGE_COLLECTION": "campus", "ARK_KNOWLEDGE_LIMIT": "0"}, wantError: "ARK_KNOWLEDGE_LIMIT"},
		{name: "有效配置", env: map[string]string{"ARK_KNOWLEDGE_ENABLED": "true", "VOLC_API_KEY": "test-key", "ARK_KNOWLEDGE_COLLECTION": "campus", "ARK_KNOWLEDGE_LIMIT": "3"}, wantLimit: 3},
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
				if test.wantError != "" {
					So(client, ShouldBeNil)
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, test.wantError)
					return
				}
				if test.wantNil {
					So(err, ShouldBeNil)
					So(client, ShouldBeNil)
					return
				}
				So(err, ShouldBeNil)
				So(client, ShouldNotBeNil)
				So(client.collection, ShouldNotBeNil)
				So(client.limit, ShouldEqual, test.wantLimit)
			})
		}
	})
}

func TestNormalizeEndpoint(t *testing.T) {
	Convey("规范化知识库 Endpoint", t, func() {
		Convey("裸域名应补充 HTTPS 协议", func() {
			endpoint, err := normalizeEndpoint("api-knowledgebase.mlp.cn-beijing.volces.com")

			So(err, ShouldBeNil)
			So(endpoint, ShouldEqual, "https://api-knowledgebase.mlp.cn-beijing.volces.com")
		})

		Convey("缺失 Host 的 Endpoint 应被拒绝", func() {
			_, err := normalizeEndpoint("https://")

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "ARK_KNOWLEDGE_DOMAIN")
		})
	})
}
