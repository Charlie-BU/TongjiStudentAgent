// Package webfetch 提供完整性优先的公开网页正文与候选链接提取能力。
package webfetch

import (
	"context"
	"errors"
)

// ExtractInput 描述待提取的公开页面地址。
type ExtractInput struct {
	URL string
}

// ExtractResponse 保存提取正文、来源标识与经过校验的候选链接。
type ExtractResponse struct {
	URL            string
	Content        string
	Source         string
	Links          []Link
	LinksTruncated bool
}

// RenderedPage 保存浏览器渲染得到的正文、最终地址与候选链接。
type RenderedPage struct {
	URL            string
	Content        string
	Links          []Link
	LinksTruncated bool
}

// Browser 定义页面渲染及正文与链接读取能力。
type Browser interface {
	// Render 读取页面渲染后的正文、地址与候选链接。
	Render(ctx context.Context, rawURL string) (*RenderedPage, error)
}

// Error 保存对外可识别的稳定错误状态。
type Error struct{ Status string }

// Error 返回错误的稳定状态文本。
func (e *Error) Error() string { return e.Status }

// Status 将提取错误映射为稳定的工具状态。
func Status(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Status
	}
	return "web_unavailable"
}
