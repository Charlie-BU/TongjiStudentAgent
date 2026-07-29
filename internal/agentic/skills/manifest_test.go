package skills

import (
	"testing"

	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCatalog(t *testing.T) {
	Convey("构建初始 Prompt 的 Skill Catalog", t, func() {
		Convey("只包含已批准 Skill 的安全元数据", func() {
			catalog, err := Catalog()

			So(err, ShouldBeNil)
			So(catalog, ShouldStartWith, "# Available Skills\n")
			So(catalog, ShouldContainSubstring, "`doc-generator`")
			So(catalog, ShouldContainSubstring, "`doc-optimizer`")
			So(catalog, ShouldNotContainSubstring, "SKILL.md")
			So(catalog, ShouldNotContainSubstring, "internal/agentic/skills")
		})

		Convey("已批准但缺少 Manifest 的 Skill 应阻止启动", func() {
			manifest := manifests[skillallowlist.DocOptimizerSkill]
			delete(manifests, skillallowlist.DocOptimizerSkill)
			t.Cleanup(func() {
				manifests[skillallowlist.DocOptimizerSkill] = manifest
			})

			catalog, err := Catalog()

			So(catalog, ShouldBeBlank)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "approved skill \"doc-optimizer\" has no manifest")
		})
	})
}
