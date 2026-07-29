package chat

import (
	"context"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
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

func TestMCPToolAllowlist(t *testing.T) {
	Convey("聊天服务的远程 MCP Tool 白名单", t, func() {
		tools := toolallowlist.MCPTools()

		Convey("只注册维护在 allowlist 中的远程工具", func() {
			So(tools, ShouldResemble, []string{toolallowlist.TongjiStudentScoreTool})
		})

		Convey("调用方修改返回值不应影响后续服务初始化", func() {
			tools[0] = "untrusted-tool"

			So(toolallowlist.MCPTools(), ShouldResemble, []string{toolallowlist.TongjiStudentScoreTool})
		})
	})
}

func TestAgentToolsIncludesAllowedStaticTools(t *testing.T) {
	Convey("聊天服务的静态系统 Tool 注册", t, func() {
		tools := systemtools.Tools()

		Convey("应注入已加白的静态系统工具", func() {
			So(tools, ShouldHaveLength, 1)
			info, err := tools[0].Info(context.Background())

			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, systemtools.LoadSkillToolName)
		})
	})
}

func TestAppendSkillCatalog(t *testing.T) {
	Convey("聊天服务的初始 System Prompt", t, func() {
		instruction, err := appendSkillCatalog("You are a helpful assistant.")

		Convey("应追加已批准 Skill 的安全 Catalog", func() {
			So(err, ShouldBeNil)
			So(instruction, ShouldStartWith, "You are a helpful assistant.\n\n# Available Skills\n")
			So(instruction, ShouldContainSubstring, "`doc-generator`")
			So(instruction, ShouldContainSubstring, "`doc-optimizer`")
		})
	})
}

func TestServiceChatRequiresRuntime(t *testing.T) {
	Convey("通过聊天服务执行对话", t, func() {
		Convey("Runtime 未初始化", func() {
			service := &Service{}
			response, err := service.Stream(context.Background(), "你好", nil)

			Convey("应返回初始化错误且不访问外部依赖", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat service is not initialized")
			})
		})
	})
}
