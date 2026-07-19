package knowledge

// SearchKnowledgeReq 表示 Ark 知识库检索请求。
type SearchKnowledgeReq struct {
	Name           string          `json:"name,omitempty"`
	Project        string          `json:"project,omitempty"`
	ResourceID     string          `json:"resource_id,omitempty"`
	Query          string          `json:"query"`
	Limit          int             `json:"limit,omitempty"`
	DenseWeight    float64         `json:"dense_weight,omitempty"`
	PreProcessing  *PreProcessing  `json:"pre_processing,omitempty"`
	PostProcessing *PostProcessing `json:"post_processing,omitempty"`
}

// PreProcessing 表示检索预处理配置。
type PreProcessing struct {
	NeedInstruction bool `json:"need_instruction,omitempty"`
}

// PostProcessing 表示检索后处理配置。
type PostProcessing struct {
	RetrieveCount int  `json:"retrieve_count,omitempty"`
	ChunkGroup    bool `json:"chunk_group,omitempty"`
}

// SearchKnowledgeRes 表示 Ark 知识库检索响应。
type SearchKnowledgeRes struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Data    *SearchKnowledgeData `json:"data,omitempty"`
}

// SearchKnowledgeData 表示检索响应数据。
type SearchKnowledgeData struct {
	ResultList []SearchResult `json:"result_list,omitempty"`
}

// SearchResult 表示生成 Agent 参考资料所需的检索切片。
type SearchResult struct {
	Content          string   `json:"content"`
	OriginalQuestion string   `json:"original_question,omitempty"`
	ChunkTitle       string   `json:"chunk_title,omitempty"`
	DocInfo          *DocInfo `json:"doc_info,omitempty"`
}

// DocInfo 表示切片所属文档信息。
type DocInfo struct {
	Title string `json:"title,omitempty"`
}
