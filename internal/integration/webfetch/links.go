package webfetch

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/Charlie-BU/TongjiStudent/internal/platform/publicurl"
	xhtml "golang.org/x/net/html"
)

const (
	// maxLinkCandidates 限制链接候选的处理数量。
	maxLinkCandidates = 200
	// maxLinks 限制最终返回的链接数量。
	maxLinks = 50
	// maxLinkTitleChars 限制链接标题的 Unicode 字符数。
	maxLinkTitleChars = 300
	// linkValidationTimeout 限制一批候选链接的校验时间。
	linkValidationTimeout = 2 * time.Second
)

// Link 表示尚未读取目标正文的候选来源链接。
type Link struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// extractLinks 解析 HTML 链接并根据页面基准地址补全相对路径。
func extractLinks(raw []byte, pageURL string) ([]Link, bool) {
	document, err := xhtml.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, false
	}
	for _, node := range findElements(document, "base") {
		if href := strings.TrimSpace(attr(node, "href")); href != "" {
			if reference, err := url.Parse(href); err == nil {
				base = base.ResolveReference(reference)
			}
			break
		}
	}
	result := make([]Link, 0)
	for _, node := range findElements(document, "a") {
		href := strings.TrimSpace(attr(node, "href"))
		if href == "" || strings.HasPrefix(href, "#") {
			continue
		}
		reference, err := url.Parse(href)
		if err != nil {
			continue
		}
		if len(result) == maxLinkCandidates {
			return result, true
		}
		title := nodeText(node, false)
		if title == "" {
			title = attr(node, "title")
		}
		result = append(result, Link{URL: base.ResolveReference(reference).String(), Title: title})
	}
	return result, false
}

// normalizeLink 规范化公开链接并解析 DuckDuckGo 跳转目标。
func normalizeLink(raw string) (string, error) {
	address, err := publicurl.PublicURL(raw)
	if err != nil {
		return "", err
	}
	parsed, _ := url.Parse(address)
	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if (host == "duckduckgo.com" || strings.HasSuffix(host, ".duckduckgo.com")) && strings.TrimRight(parsed.Path, "/") == "/l" {
		if target := parsed.Query().Get("uddg"); target != "" {
			return publicurl.PublicURL(target)
		}
	}
	return address, nil
}

// validateLinks 在数量与时间限制内校验、去重并裁剪候选链接。
func (n *publicNetwork) validateLinks(ctx context.Context, candidates []Link) ([]Link, bool) {
	ctx, cancel := context.WithTimeout(ctx, linkValidationTimeout)
	defer cancel()
	result := make([]Link, 0)
	seen := make(map[string]bool)
	hosts := make(map[string]bool)
	for index, candidate := range candidates {
		if index >= maxLinkCandidates || len(result) >= maxLinks || ctx.Err() != nil {
			return result, true
		}
		address, err := normalizeLink(candidate.URL)
		if err != nil || seen[address] {
			continue
		}
		seen[address] = true
		parsed, _ := url.Parse(address)
		host := parsed.Hostname()
		allowed, checked := hosts[host]
		if !checked {
			_, err := n.resolve(ctx, host)
			allowed = err == nil
			hosts[host] = allowed
		}
		if ctx.Err() != nil {
			return result, true
		}
		if !allowed {
			continue
		}
		title := []rune(normalizeText(candidate.Title))
		if len(title) > maxLinkTitleChars {
			title = title[:maxLinkTitleChars]
		}
		result = append(result, Link{URL: address, Title: string(title)})
	}
	return result, false
}
