package knowledge

import (
	"context"
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

func TestClientSearch(t *testing.T) {
	Convey("检索知识库", t, func() {
		content, title := "请按迎新通知办理。", "新生指南"
		collection := &fakeCollectionClient{}
		client := &Client{collection: collection, limit: 3}
		collection.searchResponse = &model.SearchKnowledgeResponse{Data: &model.SearchKnowledgeResult{ResultList: []model.PointInfo{{Content: &content, ChunkTitle: &title}}}}

		Convey("应使用 SDK SearchKnowledge 并转换为既有工具响应", func() {
			response, err := client.Search(context.Background(), "校园卡怎么办")

			So(err, ShouldBeNil)
			So(collection.searchRequest.Query, ShouldEqual, "校园卡怎么办")
			So(*collection.searchRequest.Limit, ShouldEqual, 3)
			So(*collection.searchRequest.DenseWeight, ShouldEqual, 0.5)
			So(response.Data.ResultList[0].Content, ShouldEqual, content)
			So(response.Data.ResultList[0].ChunkTitle, ShouldEqual, title)
		})

		Convey("SDK 调用失败时应保留调用上下文", func() {
			collection.err = errors.New("network unavailable")

			_, err := client.Search(context.Background(), "校园卡怎么办")

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "search knowledge")
		})

		Convey("SDK 返回业务错误码时应返回可定位错误", func() {
			collection.searchResponse = &model.SearchKnowledgeResponse{Code: 1001, Message: "invalid collection"}

			_, err := client.Search(context.Background(), "校园卡怎么办")

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "code 1001")
		})

		Convey("未初始化的客户端应被拒绝", func() {
			_, err := (*Client)(nil).Search(context.Background(), "校园卡怎么办")

			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "not initialized")
		})
	})
}
