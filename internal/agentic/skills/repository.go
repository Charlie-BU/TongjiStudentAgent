// Package skills 提供编译进 Agent 的受限 Skill 内容读取能力。
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode/utf8"
)

const maxSkillPackageBytes = 64 * 1024

// Files 仅包含 Agent 随二进制发布的 Skill 资源。
//
//go:embed all:*
var Files embed.FS

// Load 读取指定 Skill 的主手册及其嵌入式文本资源，不允许调用方指定任意文件路径。
func Load(skillID string) (string, error) {
	if !isSafeSkillID(skillID) {
		return "", fmt.Errorf("invalid skill ID")
	}
	manualPath := path.Join(skillID, "SKILL.md")
	content, err := Files.ReadFile(manualPath)
	if err != nil {
		return "", fmt.Errorf("read skill manual: %w", err)
	}
	if len(content) > maxSkillPackageBytes {
		return "", fmt.Errorf("skill package exceeds maximum size")
	}

	var builder strings.Builder
	builder.Write(content)
	size := len(content)
	hasResources := false
	err = fs.WalkDir(Files, skillID, func(resourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || resourcePath == manualPath {
			return nil
		}
		resource, err := Files.ReadFile(resourcePath)
		if err != nil {
			return err
		}
		if !utf8.Valid(resource) {
			return nil
		}
		relativePath := strings.TrimPrefix(resourcePath, skillID+"/")
		resourceSection := fmt.Sprintf("\n## %s\n\n%s\n", relativePath, resource)
		resourceHeader := ""
		if !hasResources {
			resourceHeader = "\n\n# Embedded Skill Resources\n"
		}
		if size+len(resourceHeader)+len(resourceSection) > maxSkillPackageBytes {
			return fmt.Errorf("skill package exceeds maximum size")
		}
		builder.WriteString(resourceHeader)
		builder.WriteString(resourceSection)
		size += len(resourceHeader) + len(resourceSection)
		hasResources = true
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read skill resources: %w", err)
	}
	return builder.String(), nil
}

// isSafeSkillID 检查 Skill ID 是否安全，不包含特殊字符或路径遍历风险。
func isSafeSkillID(skillID string) bool {
	trimmed := strings.TrimSpace(skillID)
	return trimmed != "" && trimmed == skillID && path.Base(trimmed) == trimmed && !strings.Contains(trimmed, `\\`)
}
