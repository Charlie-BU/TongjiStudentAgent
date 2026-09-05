package tavily

import "context"

// Search 调用固定深度的搜索接口。
func (c *Client) Search(ctx context.Context, input SearchInput) (*SearchResponse, error) {
	request := struct {
		SearchInput
		Topic  string `json:"topic"`
		Depth  string `json:"search_depth"`
		Auto   bool   `json:"auto_parameters"`
		Answer bool   `json:"include_answer"`
		Raw    bool   `json:"include_raw_content"`
		Images bool   `json:"include_images"`
	}{SearchInput: input, Topic: "general", Depth: "basic"}
	var response *SearchResponse
	if err := c.post(ctx, "/search", request, &response); err != nil {
		return nil, err
	}
	if response == nil || response.Results == nil {
		return nil, &Error{Status: "web_unavailable"}
	}
	return response, nil
}
