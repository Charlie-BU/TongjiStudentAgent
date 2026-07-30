package systemtools

import (
	"context"
	"testing"

	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	. "github.com/smartystreets/goconvey/convey"
)

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
			So(info.Name, ShouldEqual, loadskill.LoadSkillToolName)
		})
	})
}
