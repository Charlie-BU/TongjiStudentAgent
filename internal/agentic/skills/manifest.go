package skills

import (
	"fmt"
	"sort"
	"strings"

	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
)

// Manifest 是可安全注入 System Prompt 的 Skill 摘要；完整手册仍只由
// system.load_skill 按需返回。
type Manifest struct {
	ID          string
	Description string
}

var manifests = map[string]Manifest{
	skillallowlist.DocGeneratorSkill: {
		ID:          skillallowlist.DocGeneratorSkill,
		Description: "生成、补写或优化中文 Markdown 文档，保持事实准确和作者语气。触发于用户要求成文、局部改写、润色或在不改原意前提下优化文档时。",
	},
}

// Catalog 返回所有且仅有已批准 Skill 的元数据。它不暴露 Skill 路径或完整手册。
func Catalog() (string, error) {
	skillIDs := append([]string(nil), skillallowlist.Skills()...)
	sort.Strings(skillIDs)
	if len(skillIDs) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("# Available Skills\n")
	for _, skillID := range skillIDs {
		manifest, ok := manifests[skillID]
		if !ok {
			return "", fmt.Errorf("approved skill %q has no manifest", skillID)
		}
		if manifest.ID != skillID || strings.TrimSpace(manifest.Description) == "" {
			return "", fmt.Errorf("invalid manifest for approved skill %q", skillID)
		}
		if _, err := Load(skillID); err != nil {
			return "", fmt.Errorf("validate approved skill %q: %w", skillID, err)
		}

		fmt.Fprintf(&builder, "- `%s`：%s\n", manifest.ID, manifest.Description)
	}
	builder.WriteString("仅在满足触发条件时调用 `system.load_skill` 加载对应 Skill；不要猜测或访问未列出的 Skill。")
	return builder.String(), nil
}
