package arkmodel

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewFromEnvUsesResponsesAPIWithSessionCache(t *testing.T) {
	Convey("Ark 模型初始化", t, func() {
		t.Setenv("ENDPOINT_ID", "ep-test")
		t.Setenv("ENDPOINT_API_KEY", "test-api-key")
		t.Setenv("ARK_BASE_URL", "https://ark.example.test/api/v3")
		t.Setenv("ARK_BASE_URL_CN", "")

		chatModel, err := NewFromEnv(context.Background())

		Convey("应创建启用 response-chain 缓存的 Responses API 模型", func() {
			So(err, ShouldBeNil)
			So(chatModel, ShouldNotBeNil)
			responsesModel, ok := chatModel.(interface{ GetType() string })
			So(ok, ShouldBeTrue)
			So(responsesModel.GetType(), ShouldEqual, "ResponsesAPI")
			So(responseChainCacheTTLSeconds, ShouldEqual, 600)
		})
	})
}
