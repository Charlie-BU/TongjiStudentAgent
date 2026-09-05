package tavily

import (
	"context"
	"strings"
)

// Extract 提取单页正文并检查 HTTP 成功响应中的业务失败。
func (c *Client) Extract(ctx context.Context, input ExtractInput) (*ExtractResponse, error) {
	request := struct {
		URLs   []string `json:"urls"`
		Query  string   `json:"query,omitempty"`
		Chunks int      `json:"chunks_per_source,omitempty"`
		Depth  string   `json:"extract_depth"`
		Format string   `json:"format"`
		Images bool     `json:"include_images"`
	}{URLs: []string{input.URL}, Query: input.Query, Depth: "basic", Format: "markdown"}
	if input.Query != "" {
		request.Chunks = 5
	}
	var response *ExtractResponse
	if err := c.post(ctx, "/extract", request, &response); err != nil {
		return nil, err
	}
	if response == nil {
		return nil, &Error{Status: "web_unavailable"}
	}
	if len(response.FailedResults) > 0 || len(response.Results) != 1 || strings.TrimSpace(response.Results[0].Content) == "" {
		return nil, &Error{Status: "fetch_failed"}
	}
	return response, nil
}
