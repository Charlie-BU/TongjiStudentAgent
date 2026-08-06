package config

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfigFromEnv(t *testing.T) {
	Convey("会话环境配置", t, func() {
		Convey("未配置时使用受限的匿名会话默认值", func() {
			config, err := ConfigFromEnv()

			So(err, ShouldBeNil)
			So(config.AnonymousTTL, ShouldEqual, 24*time.Hour)
			So(config.AnonymousMessageLimit, ShouldEqual, 20)
			So(config.HistoryMessageLimit, ShouldEqual, 20)
		})

		Convey("允许覆盖容量与历史窗口", func() {
			t.Setenv(anonymousSessionTTLEnv, "2h")
			t.Setenv(anonymousSessionMessageLimitEnv, "8")
			t.Setenv(historyMessageLimitEnv, "12")
			config, err := ConfigFromEnv()

			So(err, ShouldBeNil)
			So(config.AnonymousTTL, ShouldEqual, 2*time.Hour)
			So(config.AnonymousMessageLimit, ShouldEqual, 8)
			So(config.HistoryMessageLimit, ShouldEqual, 12)
		})

		Convey("非法配置应在服务启动前失败", func() {
			t.Setenv(anonymousSessionTTLEnv, "zero")
			_, err := ConfigFromEnv()

			So(err, ShouldNotBeNil)
		})
	})
}
