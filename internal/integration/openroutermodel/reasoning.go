package openroutermodel

import "encoding/json"

// UnmarshalJSON 提取上游显式返回的可见摘要文本。
func (p *reasoningPart) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &p.Text)
	}
	return json.Unmarshal(data, (*reasoningPartJSON)(p))
}

// visibleReasoning 优先使用选定表示，缺失时回退到另一种可见文本。
func visibleReasoning(out item, preferred string) string {
	var summary, content string
	for _, part := range out.Summary {
		summary += part.Text
	}
	for _, part := range out.Content {
		if part.Type == "reasoning_text" || part.Type == "text" || part.Type == "output_text" {
			content += part.Text
		}
	}
	if preferred == "content" && content != "" {
		return content
	}
	if summary != "" {
		return summary
	}
	return content
}
