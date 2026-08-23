package knowledge

import (
	"context"
	"fmt"

	"github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

// Search 检索与 query 最相关的知识库切片。
func (c *Client) Search(ctx context.Context, query string) (*SearchKnowledgeRes, error) {
	if c == nil || c.collection == nil {
		return nil, fmt.Errorf("knowledge client is not initialized")
	}
	denseWeight := 0.5
	response, err := c.collection.SearchKnowledge(ctx, model.SearchKnowledgeRequest{
		Query:       query,
		Limit:       &c.limit,
		DenseWeight: &denseWeight,
		PreProcessing: map[string]interface{}{
			"need_instruction": true,
		},
		PostProcessing: map[string]interface{}{
			"retrieve_count": c.limit,
			"chunk_group":    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search knowledge: %w", err)
	}
	result := fromSDKSearchResponse(response)
	if result.Code != 0 {
		return result, fmt.Errorf("knowledge search returned code %d: %s", result.Code, result.Message)
	}
	return result, nil
}

// fromSDKSearchResponse 将 SDK 返回的知识库搜索响应转换为自定义结构体。
func fromSDKSearchResponse(response *model.SearchKnowledgeResponse) *SearchKnowledgeRes {
	if response == nil {
		return &SearchKnowledgeRes{}
	}
	result := &SearchKnowledgeRes{Code: response.Code, Message: response.Message}
	if response.Data == nil {
		return result
	}
	result.Data = &SearchKnowledgeData{ResultList: make([]SearchResult, 0, len(response.Data.ResultList))}
	for _, item := range response.Data.ResultList {
		searchResult := SearchResult{Content: stringValue(item.Content), OriginalQuestion: stringValue(item.OriginalQuestion), ChunkTitle: stringValue(item.ChunkTitle)}
		if item.DocInfo != nil {
			searchResult.DocInfo = &DocInfo{Title: stringValue(item.DocInfo.Title)}
		}
		result.Data.ResultList = append(result.Data.ResultList, searchResult)
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
