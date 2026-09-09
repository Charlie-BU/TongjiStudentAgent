package tavily

// SearchInput 描述经过工具校验的搜索条件。
type SearchInput struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results"`
	StartDate      string   `json:"start_date,omitempty"`
	EndDate        string   `json:"end_date,omitempty"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// SearchSource 保存供应商返回的来源信息。
type SearchSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// SearchResponse 保存搜索来源及诊断标识。
type SearchResponse struct {
	Results   []SearchSource `json:"results"`
	RequestID string         `json:"request_id"`
}
