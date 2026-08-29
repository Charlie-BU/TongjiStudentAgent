package config

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCORSAllowOrigins(t *testing.T) {
	Convey("CORS Origin 白名单配置", t, func() {
		Convey("应解析并规范化合法的 Origin 数组", func() {
			t.Setenv("CORS_ALLOW_ORIGINS", ` ["https://app.tongji.edu.cn/", "http://localhost:5173//"] `)

			got, err := CORSAllowOrigins()

			So(err, ShouldBeNil)
			So(got, ShouldResemble, []string{"https://app.tongji.edu.cn", "http://localhost:5173"})
		})

		Convey("未配置时不启用 CORS", func() {
			t.Setenv("CORS_ALLOW_ORIGINS", "")

			got, err := CORSAllowOrigins()

			So(err, ShouldBeNil)
			So(got, ShouldBeNil)
		})

		Convey("非 JSON 数组时应返回配置错误", func() {
			t.Setenv("CORS_ALLOW_ORIGINS", "https://app.tongji.edu.cn")

			_, err := CORSAllowOrigins()

			So(err, ShouldNotBeNil)
		})
	})
}
