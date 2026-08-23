package knowledge

import (
	"context"
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	vikingknowledge "github.com/volcengine/vikingdb-go-sdk/knowledge"
	"github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

type fakeCollectionClient struct {
	listResponse   *model.ListDocsResponse
	searchResponse *model.SearchKnowledgeResponse
	err            error
	request        model.ListDocsRequest
	searchRequest  model.SearchKnowledgeRequest
}

func (f *fakeCollectionClient) SearchKnowledge(_ context.Context, request model.SearchKnowledgeRequest, _ ...vikingknowledge.RequestOption) (*model.SearchKnowledgeResponse, error) {
	f.searchRequest = request
	return f.searchResponse, f.err
}

func (f *fakeCollectionClient) ListDocs(_ context.Context, request model.ListDocsRequest, _ ...vikingknowledge.RequestOption) (*model.ListDocsResponse, error) {
	f.request = request
	return f.listResponse, f.err
}

func TestClientListDocs(t *testing.T) {
	Convey("查询知识库文档", t, func() {
		collection := &fakeCollectionClient{listResponse: &model.ListDocsResponse{Data: &model.ListDocsResult{DocList: []model.DocInfo{{DocName: stringPtr("campus-guide")}}}}}
		client := &Client{collection: collection}

		Convey("应将 SDK 请求直接传给 CollectionClient", func() {
			result, err := client.ListDocs(context.Background(), model.ListDocsRequest{Offset: 0, Limit: 10})

			So(err, ShouldBeNil)
			So(collection.request.Offset, ShouldEqual, 0)
			So(collection.request.Limit, ShouldEqual, 10)
			So(result.Data.DocList, ShouldHaveLength, 1)
			So(*result.Data.DocList[0].DocName, ShouldEqual, "campus-guide")
		})

		Convey("SDK 返回错误时应保留调用上下文", func() {
			collection.err = errors.New("network unavailable")
			_, err := client.ListDocs(context.Background(), model.ListDocsRequest{})

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "list docs")
		})

		Convey("SDK 返回业务错误码时应返回可定位错误", func() {
			collection.listResponse = &model.ListDocsResponse{Code: 1001, Message: "invalid collection"}

			_, err := client.ListDocs(context.Background(), model.ListDocsRequest{})

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "code 1001")
		})

		Convey("未初始化的客户端应被拒绝", func() {
			_, err := (*Client)(nil).ListDocs(context.Background(), model.ListDocsRequest{})

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "not initialized")
		})
	})
}

func stringPtr(value string) *string { return &value }
