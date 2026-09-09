package searchknowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeSearcher struct {
	response *knowledge.SearchKnowledgeRes
	err      error
	query    string
}

func (f *fakeSearcher) Search(_ context.Context, query string) (*knowledge.SearchKnowledgeRes, error) {
	f.query = query
	return f.response, f.err
}

func TestSearchKnowledgeTool(t *testing.T) {
	Convey("校园知识库系统工具", t, func() {
		searcher := &fakeSearcher{response: &knowledge.SearchKnowledgeRes{Data: &knowledge.SearchKnowledgeData{ResultList: []knowledge.SearchResult{
			{ChunkTitle: "新生指南", OriginalQuestion: "校园卡怎么办", Content: "请按迎新通知办理。"},
		}}}}
		tool := NewTool(func(name string) bool { return name == SearchKnowledgeToolName }, searcher)

		Convey("应声明适用范围、串行限制与个人数据边界", func() {
			info, err := tool.Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, SearchKnowledgeToolName)
			So(info.Desc, ShouldContainSubstring, "官方依据")
			So(info.Desc, ShouldContainSubstring, "收到上一次结果后才能发起下一次")
			So(info.Desc, ShouldContainSubstring, "即使知识库命中也不能省略网页收集")
			So(info.Desc, ShouldContainSubstring, "第一可信来源")
			So(info.Desc, ShouldContainSubstring, "个人实时数据")
		})

		Convey("成功检索时应返回结构化且可追溯的资料", func() {
			actual, err := tool.InvokableRun(context.Background(), `{"query":"新生校园卡怎么办","reason":"需要核验办理流程"}`)
			So(err, ShouldBeNil)
			So(searcher.query, ShouldEqual, "新生校园卡怎么办")
			var output result
			So(json.Unmarshal([]byte(actual), &output), ShouldBeNil)
			So(output.Status, ShouldEqual, "ok")
			So(output.Sources, ShouldHaveLength, 1)
			So(output.Sources[0].Title, ShouldEqual, "新生指南")
			So(output.Sources[0].OriginalQuestion, ShouldEqual, "校园卡怎么办")
		})

		Convey("未知字段和缺失必填参数应被拒绝", func() {
			actual, err := tool.InvokableRun(context.Background(), `{"query":"校园卡","reason":"核验","limit":10}`)
			So(err, ShouldBeNil)
			So(actual, ShouldContainSubstring, `"status":"invalid_arguments"`)

			actual, err = tool.InvokableRun(context.Background(), `{"query":"校园卡","reason":""}`)
			So(err, ShouldBeNil)
			So(actual, ShouldContainSubstring, `"status":"invalid_arguments"`)
		})

		Convey("无结果和可恢复的检索失败应返回稳定状态", func() {
			searcher.response = &knowledge.SearchKnowledgeRes{}
			actual, err := tool.InvokableRun(context.Background(), `{"query":"未知事项","reason":"核验"}`)
			So(err, ShouldBeNil)
			So(actual, ShouldContainSubstring, `"status":"no_results"`)

			searcher.err = errors.New("network unavailable")
			actual, err = tool.InvokableRun(context.Background(), `{"query":"校历","reason":"核验"}`)
			So(err, ShouldBeNil)
			So(actual, ShouldContainSubstring, `"status":"knowledge_unavailable"`)
		})

		Convey("应限制返回条数并截断超长资料", func() {
			searcher.err = nil
			searcher.response = &knowledge.SearchKnowledgeRes{Data: &knowledge.SearchKnowledgeData{ResultList: []knowledge.SearchResult{
				{Content: strings.Repeat("知", maxContentRunes+1)}, {Content: "2"}, {Content: "3"}, {Content: "4"}, {Content: "5"}, {Content: "6"},
			}}}
			actual, err := tool.InvokableRun(context.Background(), `{"query":"校园服务","reason":"核验"}`)
			So(err, ShouldBeNil)
			var output result
			So(json.Unmarshal([]byte(actual), &output), ShouldBeNil)
			So(output.Sources, ShouldHaveLength, maxSources)
			So(output.Sources[0].Content, ShouldEndWith, "…")
		})
	})
}
