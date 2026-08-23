package knowledge

import (
	"context"
	"fmt"

	"github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

// ListDocs 分页查询当前知识库的文档。
func (c *Client) ListDocs(ctx context.Context, request model.ListDocsRequest) (*model.ListDocsResponse, error) {
	if c == nil || c.collection == nil {
		return nil, fmt.Errorf("knowledge client is not initialized")
	}
	response, err := c.collection.ListDocs(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("list docs: %w", err)
	}
	if response != nil && response.Code != 0 {
		return response, fmt.Errorf("list docs returned code %d: %s", response.Code, response.Message)
	}
	return response, nil
}
