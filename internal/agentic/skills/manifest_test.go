package skills

import (
	"path"
	"sort"
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

func TestSkillCatalogAllowlistAndEmbeddedFilesStayConsistent(t *testing.T) {
	Convey("Skill Catalog、allowlist 与嵌入手册", t, func() {
		catalog, err := Catalog()
		allowlistedIDs := skillallowlist.Skills()
		embeddedIDs, embeddedErr := embeddedSkillIDs()
		manifestIDs := make([]string, 0, len(manifests))
		for skillID := range manifests {
			manifestIDs = append(manifestIDs, skillID)
		}
		sort.Strings(allowlistedIDs)
		sort.Strings(manifestIDs)

		Convey("三处注册集合必须完全一致", func() {
			So(err, ShouldBeNil)
			So(embeddedErr, ShouldBeNil)
			So(manifestIDs, ShouldResemble, allowlistedIDs)
			So(embeddedIDs, ShouldResemble, allowlistedIDs)
		})

		Convey("每个已批准 Skill 必须出现在 Catalog 中", func() {
			for _, skillID := range allowlistedIDs {
				So(catalog, ShouldContainSubstring, "- `"+skillID+"`")
			}
		})
	})
}

func embeddedSkillIDs() ([]string, error) {
	entries, err := Files.ReadDir(".")
	if err != nil {
		return nil, err
	}

	skillIDs := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := Files.ReadFile(path.Join(entry.Name(), "SKILL.md")); err == nil {
			skillIDs = append(skillIDs, entry.Name())
		}
	}
	sort.Strings(skillIDs)
	return skillIDs, nil
}
