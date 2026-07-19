package sandbox

import (
	"testing"

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
