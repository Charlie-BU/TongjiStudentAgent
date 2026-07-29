package skills

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoad(t *testing.T) {
	Convey("读取编译进 Agent 的 Skill", t, func() {
		Convey("合法 Skill 应返回主手册", func() {
			content, err := Load("doc-generator")

			So(err, ShouldBeNil)
			So(content, ShouldContainSubstring, "文档")
		})

		Convey("空白或包含路径的 Skill ID 应被拒绝", func() {
			for _, skillID := range []string{"", " doc-generator", "doc-generator ", "../doc-generator", "doc-generator/../doc-generator"} {
				content, err := Load(skillID)

				So(content, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "invalid skill ID")
			}
		})

		Convey("不存在的合法格式 Skill 应返回读取错误", func() {
			content, err := Load("unknown")

			So(content, ShouldBeBlank)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "read skill manual")
		})
	})
}
