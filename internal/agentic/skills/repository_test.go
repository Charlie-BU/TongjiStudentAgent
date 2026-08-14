package skills

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoad(t *testing.T) {
	Convey("读取编译进 Agent 的 Skill", t, func() {
		Convey("合法 Skill 应返回主手册及其文本参考资源", func() {
			content, err := Load("doc-generator")

			So(err, ShouldBeNil)
			So(content, ShouldContainSubstring, "文档")
			So(content, ShouldContainSubstring, "# Embedded Skill Resources")
			So(content, ShouldContainSubstring, "references/ARTICLE1.md")
			So(content, ShouldContainSubstring, "HeartCompass 向 Immortality 的重构过程")
			So(content, ShouldContainSubstring, "references/ARTICLE2.md")
			So(content, ShouldContainSubstring, "那段高三与高考记忆")
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
