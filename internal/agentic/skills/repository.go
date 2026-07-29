// Package skills 提供编译进 Agent 的受限 Skill 内容读取能力。
package skills

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

const maxSkillContentBytes = 64 * 1024

// Files 仅包含 Agent 随二进制发布的 Skill 资源。
//
//go:embed all:*
var Files embed.FS

// Load 读取指定 Skill 的主手册，不允许调用方指定任意文件路径。
func Load(skillID string) (string, error) {
	if !isSafeSkillID(skillID) {
		return "", fmt.Errorf("invalid skill ID")
	}
	content, err := Files.ReadFile(path.Join(skillID, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("read skill manual: %w", err)
	}
	if len(content) > maxSkillContentBytes {
		return "", fmt.Errorf("skill manual exceeds maximum size")
	}
	return string(content), nil
}

func isSafeSkillID(skillID string) bool {
	trimmed := strings.TrimSpace(skillID)
	return trimmed != "" && trimmed == skillID && path.Base(trimmed) == trimmed && !strings.Contains(trimmed, `\\`)
}
