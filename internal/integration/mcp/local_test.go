package mcp

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetEinoToolsFromLocalMCPServer(t *testing.T) {
	Convey("从本地 MCP Server 获取 Eino 工具", t, func() {
		ctx := context.Background()
		client, err := NewLocalClient(ctx)
		So(err, ShouldBeNil)
		So(client, ShouldNotBeNil)
		defer client.Close()

		Convey("转换工具目录", func() {
			tools, err := EinoTools(ctx, client)

			Convey("应得到一个可调用工具", func() {
				So(err, ShouldBeNil)
				So(tools, ShouldHaveLength, 1)

				invokable, ok := tools[0].(tool.InvokableTool)
				So(ok, ShouldBeTrue)
				result, err := invokable.InvokableRun(ctx, "{}")
				So(err, ShouldBeNil)
				So(result, ShouldNotBeBlank)
			})
		})
	})
}
