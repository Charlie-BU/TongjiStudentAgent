package loadskill

import (
	"context"
	"encoding/json"
	"testing"

	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	. "github.com/smartystreets/goconvey/convey"
)

// TestCourseAndTeacherGuideSkill 验证课程与教师指南技能的实际注册、嵌入加载与 Run 隔离。
func TestCourseAndTeacherGuideSkill(t *testing.T) {
	Convey("课程与教师指南技能可由运行时发现并加载", t, func() {
		skillID := skillallowlist.CourseAndTeacherGuideSkill
		tool := NewTool(toolallowlist.IsAllowedTool)
		args, err := json.Marshal(map[string]string{"skill_id": skillID, "reason": "查询测试课程信息并聚合课程与教师评价"})
		So(err, ShouldBeNil)
		manual, err := agenticskills.Load(skillID)
		So(err, ShouldBeNil)
		So(len(manual), ShouldBeLessThanOrEqualTo, 64*1024)
		catalog, err := agenticskills.Catalog()
		So(err, ShouldBeNil)
		So(catalog, ShouldContainSubstring, "`"+skillID+"`")
		So(skillallowlist.IsAllowedSkill(skillID), ShouldBeTrue)
		So(skillallowlist.IsAllowedSkill("teacher-reviews"), ShouldBeFalse)

		Convey("课程指南依赖的全部课程工具均可调用", func() {
			for _, name := range []string{
				toolallowlist.TongjiCourseCatalogTool,
				toolallowlist.TongjiCourseDetailTool,
				toolallowlist.TongjiCourseRelatedTool,
				toolallowlist.TongjiCourseReviewsTool,
				toolallowlist.TongjiCourseSummaryTool,
				toolallowlist.TongjiLegacyTeacherReviewsTool,
			} {
				So(toolallowlist.IsAllowedTool(name), ShouldBeTrue)
			}
		})

		Convey("首次加载返回完整嵌入资源且同一 Run 不重复注入", func() {
			ctx := agenticskills.WithRunState(context.Background(), agenticskills.NewRunState())
			result, loadErr := tool.InvokableRun(ctx, string(args))
			So(loadErr, ShouldBeNil)
			var payload map[string]string
			So(json.Unmarshal([]byte(result), &payload), ShouldBeNil)
			So(payload["status"], ShouldEqual, "ok")
			So(payload["skill_id"], ShouldEqual, skillID)
			So(payload["content"], ShouldEqual, manual)

			again, againErr := tool.InvokableRun(ctx, string(args))
			So(againErr, ShouldBeNil)
			var duplicate map[string]string
			So(json.Unmarshal([]byte(again), &duplicate), ShouldBeNil)
			So(duplicate["status"], ShouldEqual, "already_loaded")
			_, hasContent := duplicate["content"]
			So(hasContent, ShouldBeFalse)
		})

		Convey("不同 Run 均可加载完整课程与教师指南技能", func() {
			for i := 0; i < 2; i++ {
				ctx := agenticskills.WithRunState(context.Background(), agenticskills.NewRunState())
				result, loadErr := tool.InvokableRun(ctx, string(args))
				So(loadErr, ShouldBeNil)
				var payload map[string]string
				So(json.Unmarshal([]byte(result), &payload), ShouldBeNil)
				So(payload["content"], ShouldEqual, manual)
			}
		})

		Convey("历史工具注册当前名称且不隐式开启旧别名", func() {
			So(toolallowlist.TongjiLegacyTeacherReviewsTool, ShouldEqual, "tongji.course.legacy-teacher-reviews")
			So(toolallowlist.MCPTools(), ShouldContain, toolallowlist.TongjiLegacyTeacherReviewsTool)
			So(toolallowlist.IsAllowedTool(toolallowlist.TongjiLegacyTeacherReviewsTool), ShouldBeTrue)
			So(toolallowlist.IsAllowedTool("tongji.student.legacy-teacher-reviews"), ShouldBeFalse)
			So(toolallowlist.ValidateToolAllowlist(toolallowlist.MCPTools()), ShouldBeNil)
		})
	})
}
