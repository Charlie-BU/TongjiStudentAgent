package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetCurrentTimeTool(t *testing.T) {
	Convey("本地 MCP 当前时间工具", t, func() {
		ctx := context.Background()
		client, err := NewLocalClient(ctx)
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)
		defer client.Close()

		Convey("调用 get_current_time", func() {
			result, err := client.CallTool(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{Name: "get_current_time"},
			})

			Convey("应返回非空且无错误的工具结果", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(result.IsError, ShouldBeFalse)
				So(result.Content, ShouldNotBeEmpty)
			})
		})
	})
}
