package runtime

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRequiresChatModel(t *testing.T) {
	Convey("创建 Runtime", t, func() {
		Convey("未提供 ChatModel", func() {
			runtime, err := New(context.Background(), Config{})

			Convey("应拒绝配置并返回可定位错误", func() {
				So(runtime, ShouldBeNil)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat model is required")
			})
		})
	})
}

func TestChatRequiresInitializedRuntime(t *testing.T) {
	Convey("执行 Runtime 聊天", t, func() {
		Convey("Runtime 未初始化", func() {
			var runtime *Runtime
			response, err := runtime.Chat(context.Background(), "你好")

			Convey("应返回初始化错误且不产生响应", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "agent runtime is not initialized")
			})
		})
	})
}
