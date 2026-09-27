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

func TestServerPort(t *testing.T) {
	t.Setenv("APP_PORT", "")
	t.Setenv("PORT", "10022") // GitLab SSH port must not change the HTTP listener.
	t.Setenv("PORT0", "1234")
	if actual := ServerPort(); actual != "8080" {
		t.Fatalf("ServerPort() = %q, want default 8080", actual)
	}

	t.Setenv("APP_PORT", "9080")
	if actual := ServerPort(); actual != "9080" {
		t.Fatalf("ServerPort() = %q, want APP_PORT", actual)
	}
}
