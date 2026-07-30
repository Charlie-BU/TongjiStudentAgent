// Package skill 集中维护允许调用的 skill 白名单。
package skill

const (
	DocGeneratorSkill = "doc-generator"
)

var (
	allowedSkills = []string{DocGeneratorSkill}
)

// Skills 返回已批准 Skill 标识的副本，调用方修改结果不会影响 allowlist。
func Skills() []string {
	return append([]string(nil), allowedSkills...)
}

// IsAllowedSkill 判断 Skill 是否已被应用 allowlist 明确批准。
func IsAllowedSkill(skillID string) bool {
	for _, allowedSkill := range allowedSkills {
		if skillID == allowedSkill {
			return true
		}
	}
	return false
}
