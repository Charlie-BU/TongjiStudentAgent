package skills

import (
	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"strings"
	"testing"
)

func TestCampusSkillCoversAllAllowedCampusTools(t *testing.T) {
	manual, err := Load(skillallowlist.TongjiCampusToolsSkill)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, name := range toolallowlist.MCPTools() {
		if !strings.HasPrefix(name, "tongji.") || strings.HasPrefix(name, "tongji.course.") {
			continue
		}
		count++
		if !strings.Contains(manual, "### `"+name+"`") {
			t.Errorf("missing tool guide: %s", name)
		}
	}
	if count != 42 {
		t.Fatalf("unexpected campus tool count: %d", count)
	}
}
