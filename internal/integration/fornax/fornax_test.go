package fornax

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "显式开启", value: "true", want: true},
		{name: "数字开启", value: "1", want: true},
		{name: "显式关闭", value: "false", want: false},
		{name: "未设置", want: false},
		{name: "非法值", value: "enabled", want: false},
	}

	Convey("可选 Trace 集成开关", t, func() {
		for _, test := range tests {
			test := test
			Convey(test.name, func() {
				t.Setenv("FORNAX_ENABLED", test.value)

				Convey("应解析为预期状态", func() {
					So(Enabled(), ShouldEqual, test.want)
				})
			})
		}
	})
}
