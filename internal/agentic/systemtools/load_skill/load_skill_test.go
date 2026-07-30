package loadskill

import (
	"context"
	"testing"

	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadSkillTool(t *testing.T) {
	Convey("加载受限 Skill", t, func() {
		tool := NewTool(func(string) bool { return true })
		newRunContext := func() context.Context {
			return agenticskills.WithRunState(context.Background(), agenticskills.NewRunState())
		}

		Convey("已加白的 Skill 应返回嵌入式手册", func() {
			result, err := tool.InvokableRun(newRunContext(), `{"skill_id":"doc-generator","reason":"需要生成文档"}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"ok"`)
			So(result, ShouldContainSubstring, "文档")
			So(result, ShouldNotContainSubstring, "/Users/")
		})

		Convey("未加白或越权的 Skill 不应被读取", func() {
			for _, skillID := range []string{"unknown", "../doc-generator", "doc-generator/../doc-generator"} {
				result, err := tool.InvokableRun(newRunContext(), `{"skill_id":"`+skillID+`","reason":"test"}`)

				So(err, ShouldBeNil)
				So(result, ShouldContainSubstring, `"status":"skill_not_allowed"`)
			}
		})

		Convey("参数形状不合法时不应读取任何文件", func() {
			result, err := tool.InvokableRun(newRunContext(), `{"skill_id":"doc-generator","reason":"test","path":"/etc/passwd"}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"invalid_arguments"`)
		})

		Convey("同一 Run 重复加载不应再次返回完整手册", func() {
			ctx := newRunContext()
			first, firstErr := tool.InvokableRun(ctx, `{"skill_id":"doc-generator","reason":"需要生成文档"}`)
			second, secondErr := tool.InvokableRun(ctx, `{"skill_id":"doc-generator","reason":"再次确认"}`)

			So(firstErr, ShouldBeNil)
			So(first, ShouldContainSubstring, `"status":"ok"`)
			So(secondErr, ShouldBeNil)
			So(second, ShouldContainSubstring, `"status":"already_loaded"`)
			So(second, ShouldNotContainSubstring, `"content"`)
		})

		Convey("未绑定 Run State 时应拒绝加载", func() {
			result, err := tool.InvokableRun(context.Background(), `{"skill_id":"doc-generator","reason":"需要生成文档"}`)

			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, `"status":"skill_run_unavailable"`)
		})
	})
}
