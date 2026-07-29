package systemtools

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadSkillTool(t *testing.T) {
	Convey("加载受限 Skill", t, func() {
		tool := newLoadSkillTool(func(string) bool { return true })

		Convey("已加白的 Skill 应返回嵌入式手册", func() {
			result, err := tool.InvokableRun(context.Background(), `{"skill_id":"doc-generator","reason":"需要生成文档"}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"ok"`)
			So(result, ShouldContainSubstring, "文档")
			So(result, ShouldNotContainSubstring, "/Users/")
		})

		Convey("未加白或越权的 Skill 不应被读取", func() {
			for _, skillID := range []string{"unknown", "../doc-generator", "doc-generator/../doc-generator"} {
				result, err := tool.InvokableRun(context.Background(), `{"skill_id":"`+skillID+`","reason":"test"}`)

				So(err, ShouldBeNil)
				So(result, ShouldContainSubstring, `"status":"skill_not_allowed"`)
			}
		})

		Convey("参数形状不合法时不应读取任何文件", func() {
			result, err := tool.InvokableRun(context.Background(), `{"skill_id":"doc-generator","reason":"test","path":"/etc/passwd"}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"invalid_arguments"`)
		})
	})
}

func TestSystemToolsRequireToolAllowlist(t *testing.T) {
	Convey("静态系统工具注册", t, func() {
		Convey("未加白工具不应注册", func() {
			So(buildTools(func(string) bool { return false }), ShouldBeEmpty)
		})

		Convey("已加白工具应注册", func() {
			tools := Tools()

			So(tools, ShouldHaveLength, 1)
			info, err := tools[0].Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, LoadSkillToolName)
		})
	})
}
