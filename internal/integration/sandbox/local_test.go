package sandbox

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "unset", want: false},
		{name: "disabled", value: "false", want: false},
		{name: "enabled", value: "true", want: true},
		{name: "numeric enabled", value: "1", want: true},
		{name: "invalid", value: "enabled", wantErr: true},
	}

	Convey("Sandbox 开关解析", t, func() {
		for _, test := range tests {
			test := test
			Convey(test.name, func() {
				t.Setenv("SANDBOX_ENABLED", test.value)
				enabled, err := EnabledFromEnv()

				if test.wantErr {
					Convey("应拒绝非法配置", func() {
						So(enabled, ShouldBeFalse)
						So(err, ShouldNotBeNil)
						So(err.Error(), ShouldContainSubstring, "parse SANDBOX_ENABLED")
					})
					return
				}

				Convey("应返回预期的启用状态", func() {
					So(err, ShouldBeNil)
					So(enabled, ShouldEqual, test.want)
				})
			})
		}
	})
}

func TestNewFileSystemMiddleware(t *testing.T) {
	Convey("创建文件系统中间件", t, func() {
		middleware, err := NewFileSystemMiddleware(context.Background())

		Convey("应完成本地 Backend 装配且不执行 Shell", func() {
			So(err, ShouldBeNil)
			So(middleware, ShouldNotBeNil)

			_, agentContext, beforeErr := middleware.BeforeAgent(context.Background(), &adk.ChatModelAgentContext{})
			So(beforeErr, ShouldBeNil)
			So(agentContext.Tools, ShouldNotBeEmpty)
		})
	})
}
