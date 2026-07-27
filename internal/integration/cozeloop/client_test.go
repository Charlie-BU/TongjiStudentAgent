package cozeloop

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "显式开启", value: "true", want: true},
		{name: "数字开启", value: "1", want: true},
		{name: "显式关闭", value: "false", want: false},
		{name: "未设置", want: false},
		{name: "非法值", value: "enabled", want: false},
	}

	Convey("可选 Cozeloop 集成开关", t, func() {
		for _, test := range tests {
			Convey(test.name, func() {
				t.Setenv("COZELOOP_ENABLED", test.value)

				Convey("应解析为预期状态", func() {
					So(Enabled(), ShouldEqual, test.want)
				})
			})
		}
	})
}

func TestInitRequiresJWTPrivateKey(t *testing.T) {
	Convey("初始化 Cozeloop", t, func() {
		t.Setenv("COZELOOP_ENABLED", "true")
		t.Setenv("COZELOOP_WORKSPACE_ID", "test-workspace")
		t.Setenv("COZELOOP_JWT_OAUTH_CLIENT_ID", "test-client")
		t.Setenv("COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID", "test-key-id")
		t.Setenv("COZELOOP_JWT_OAUTH_PRIVATE_KEY", "")

		Convey("缺少 JWT 私钥", func() {
			err := Init(context.Background(), nil)

			Convey("应在创建客户端前返回明确配置错误", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "COZELOOP_JWT_OAUTH_PRIVATE_KEY")
			})
		})
	})
}

func TestFetchPromptRequiresInitializedClient(t *testing.T) {
	Convey("拉取 PromptHub 提示词", t, func() {
		original := client
		client = nil
		t.Cleanup(func() { client = original })

		messages, err := FetchPrompt(context.Background(), "test.prompt", "1.0.0", map[string]any{"name": "Alice"})

		So(messages, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "client is not initialized")
	})
}

func TestMessageContent(t *testing.T) {
	Convey("提取 PromptHub 消息内容", t, func() {
		Convey("按角色拼接文本消息", func() {
			content, err := MessageContent([]*schema.Message{
				schema.UserMessage("ignored"),
				schema.SystemMessage(" first instruction "),
				schema.SystemMessage("second instruction"),
			}, schema.System)

			So(err, ShouldBeNil)
			So(content, ShouldEqual, "first instruction\n\nsecond instruction")
		})

		Convey("指定角色不存在", func() {
			content, err := MessageContent([]*schema.Message{schema.UserMessage("not a system prompt")}, schema.System)

			So(content, ShouldBeBlank)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "contains no \"system\" message")
		})
	})
}
