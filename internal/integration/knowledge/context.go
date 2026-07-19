package knowledge

import (
	"fmt"
	"strings"
)

// FormatContext 将检索切片转换为供 Agent 使用的非可信参考资料。
func FormatContext(response *SearchKnowledgeRes) string {
	if response == nil || response.Data == nil || len(response.Data.ResultList) == 0 {
		return ""
	}

	var builder strings.Builder
	for index, item := range response.Data.ResultList {
		content := item.Content
		if item.OriginalQuestion != "" {
			content = fmt.Sprintf("相似问题：%s\n参考答案：%s", item.OriginalQuestion, item.Content)
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		title := item.ChunkTitle
		if title == "" && item.DocInfo != nil {
			title = item.DocInfo.Title
		}
		if title != "" {
			fmt.Fprintf(&builder, "[%d] %s\n", index+1, title)
		}
		builder.WriteString(content)
		builder.WriteString("\n---\n")
	}
	return strings.TrimSuffix(builder.String(), "---\n")
}
