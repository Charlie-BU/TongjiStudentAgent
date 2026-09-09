package webfetch

import (
	"bytes"
	"encoding/json"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

var (
	// noscriptPattern 匹配页面的无脚本备用内容。
	noscriptPattern = regexp.MustCompile(`(?is)<noscript(?:\s[^>]*)?>(.*?)</noscript>`)
	// jsonScriptPattern 匹配嵌入 JSON 数据的脚本内容。
	jsonScriptPattern = regexp.MustCompile(`(?is)<script[^>]*type=["']application/json["'][^>]*>(.*?)</script>`)
	// richHTMLPattern 识别可能承载正文的富 HTML 字符串。
	richHTMLPattern = regexp.MustCompile(`(?i)<(?:main|p|h[1-6]|li|table|article|section)\b`)
)

// contentCandidate 保存一种页面表示的正文及来源标识。
type contentCandidate struct {
	source string
	text   string
}

// normalizeText 归一化实体与空白并保留非空行的顺序和重复内容。
func normalizeText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r", ""), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(html.UnescapeString(line)), " ")
		if line != "" {
			result = append(result, line)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// mergeText 合并不同正文表示并消除包含片段及充分的首尾重叠。
func mergeText(values ...string) string {
	var merged string
	for _, value := range values {
		next := normalizeText(value)
		if next == "" {
			continue
		}
		if merged == "" {
			merged = next
			continue
		}
		if strings.Contains("\n"+merged+"\n", "\n"+next+"\n") {
			continue
		}
		if strings.Contains("\n"+next+"\n", "\n"+merged+"\n") {
			merged = next
			continue
		}
		left, right := strings.Split(merged, "\n"), strings.Split(next, "\n")
		overlap := suffixPrefixOverlap(left, right)
		// 保留孤立短值，避免误合并不同上下文中的相同事实。
		if overlap == 1 && utf8.RuneCountInString(right[0]) < 80 {
			overlap = 0
		}
		merged += "\n" + strings.Join(right[overlap:], "\n")
	}
	return merged
}

// suffixPrefixOverlap 计算两组文本行之间的最长后缀与前缀重叠长度。
func suffixPrefixOverlap(left, right []string) int {
	prefix := make([]int, len(right))
	for i, j := 1, 0; i < len(right); i++ {
		for j > 0 && right[i] != right[j] {
			j = prefix[j-1]
		}
		if right[i] == right[j] {
			j++
		}
		prefix[i] = j
	}
	matched := 0
	for _, line := range left {
		for matched > 0 && (matched == len(right) || line != right[matched]) {
			matched = prefix[matched-1]
		}
		if line == right[matched] {
			matched++
		}
	}
	return matched
}

// mergeCandidates 汇总候选正文并生成去重后的来源标识。
func mergeCandidates(candidates []contentCandidate) (string, string) {
	texts := make([]string, 0, len(candidates))
	sources := make([]string, 0, len(candidates))
	seenSources := make(map[string]struct{})
	for _, candidate := range candidates {
		texts = append(texts, candidate.text)
		if _, exists := seenSources[candidate.source]; !exists {
			seenSources[candidate.source] = struct{}{}
			sources = append(sources, candidate.source)
		}
	}
	return mergeText(texts...), strings.Join(sources, "+")
}

// skipElement 判断正文提取是否应跳过指定元素。
func skipElement(name string, includeNoscript bool) bool {
	switch name {
	case "script", "style", "template", "svg", "canvas", "iframe":
		return true
	case "noscript":
		return !includeNoscript
	default:
		return false
	}
}

// blockElement 判断元素是否需要在文本边界插入换行。
func blockElement(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "td", "th", "tr", "ul":
		return true
	default:
		return false
	}
}

// nodeText 提取节点子树中的正文并保留块级换行。
func nodeText(root *xhtml.Node, includeNoscript bool) string {
	var output strings.Builder
	var visit func(*xhtml.Node)
	visit = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && skipElement(node.Data, includeNoscript) {
			return
		}
		if node.Type == xhtml.TextNode {
			output.WriteString(node.Data)
			output.WriteByte(' ')
			return
		}
		isBlock := node.Type == xhtml.ElementNode && blockElement(node.Data)
		if isBlock {
			output.WriteByte('\n')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
		if isBlock {
			output.WriteByte('\n')
		}
	}
	visit(root)
	return normalizeText(output.String())
}

// parseText 将 HTML 片段转换为归一化正文。
func parseText(fragment string) string {
	document, err := xhtml.Parse(strings.NewReader(fragment))
	if err != nil {
		return ""
	}
	return nodeText(document, true)
}

// attr 读取节点中指定名称的属性值。
func attr(node *xhtml.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

// findElements 查找匹配标签名或角色的节点。
func findElements(root *xhtml.Node, names ...string) []*xhtml.Node {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	var result []*xhtml.Node
	var visit func(*xhtml.Node)
	visit = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && (wanted[node.Data] || wanted["role="+strings.ToLower(attr(node, "role"))]) {
			result = append(result, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return result
}

// hydrationStrings 按确定顺序收集嵌入数据中的富 HTML 字符串。
func hydrationStrings(value any, output *[]string) {
	switch typed := value.(type) {
	case string:
		if len(typed) >= 80 && richHTMLPattern.MatchString(typed) {
			*output = append(*output, typed)
		}
	case []any:
		for _, item := range typed {
			hydrationStrings(item, output)
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			hydrationStrings(typed[key], output)
		}
	}
}

// candidatePriority 返回不同正文来源的合并优先级。
func candidatePriority(source string) int {
	switch source {
	case "http_main":
		return 0
	case "http_hydration_json":
		return 1
	case "http_noscript":
		return 2
	case "http_body":
		return 3
	default:
		return 4
	}
}

// extractCandidates 提取并排序 HTML、嵌入数据与备用页面中的正文候选。
func extractCandidates(raw []byte) []contentCandidate {
	document, _ := xhtml.Parse(bytes.NewReader(raw))
	candidates := make([]contentCandidate, 0, 8)
	if document != nil {
		for _, node := range findElements(document, "article", "main", "role=main") {
			if text := nodeText(node, false); text != "" {
				candidates = append(candidates, contentCandidate{source: "http_main", text: text})
			}
		}
		if bodies := findElements(document, "body"); len(bodies) > 0 {
			if text := nodeText(bodies[0], false); text != "" {
				candidates = append(candidates, contentCandidate{source: "http_body", text: text})
			}
		}
	}

	for _, match := range noscriptPattern.FindAllSubmatch(raw, -1) {
		if text := parseText(string(match[1])); text != "" {
			candidates = append(candidates, contentCandidate{source: "http_noscript", text: text})
		}
	}
	for _, match := range jsonScriptPattern.FindAllSubmatch(raw, -1) {
		var payload any
		if json.Unmarshal(bytes.TrimSpace(match[1]), &payload) != nil {
			continue
		}
		var values []string
		hydrationStrings(payload, &values)
		parts := make([]string, 0, len(values))
		for _, value := range values {
			if text := parseText(value); text != "" {
				parts = append(parts, text)
			}
		}
		if text := normalizeText(strings.Join(parts, "\n")); text != "" {
			candidates = append(candidates, contentCandidate{source: "http_hydration_json", text: text})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if left, right := candidatePriority(candidates[i].source), candidatePriority(candidates[j].source); left != right {
			return left < right
		}
		return utf8.RuneCountInString(candidates[i].text) > utf8.RuneCountInString(candidates[j].text)
	})
	return candidates
}
