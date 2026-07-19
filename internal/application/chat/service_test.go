package chat

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestChatRequiresInitializedDefaultService(t *testing.T) {
	Convey("使用默认聊天服务", t, func() {
		original := defaultService
		defaultService = nil
		t.Cleanup(func() { defaultService = original })

		Convey("默认服务未初始化", func() {
			response, err := Chat(context.Background(), "你好")

			Convey("应返回初始化错误且不产生响应", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat service is not initialized")
			})
		})
	})
}

func TestServiceWithKnowledgeContextWithoutKnowledgeClient(t *testing.T) {
	Convey("组装知识库上下文", t, func() {
		service := &Service{}

		Convey("知识库未启用", func() {
			input, err := service.withKnowledgeContext(context.Background(), "图书馆在哪里？")

			Convey("应原样传递用户问题", func() {
				So(err, ShouldBeNil)
				So(input, ShouldEqual, "图书馆在哪里？")
			})
		})
	})
}

func TestServiceChatRequiresRuntime(t *testing.T) {
	Convey("通过聊天服务执行对话", t, func() {
		Convey("Runtime 未初始化", func() {
			service := &Service{}
			response, err := service.Chat(context.Background(), "你好")

			Convey("应返回初始化错误且不访问外部依赖", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat service is not initialized")
			})
		})
	})
}
