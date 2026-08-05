package session

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
			So(config.ContextTokenBudget, ShouldEqual, 6000)
			So(config.SummaryMaxTokens, ShouldEqual, 1200)
			So(config.SummaryRecentTurns, ShouldEqual, 2)
			So(config.SummaryScanMaxMessages, ShouldEqual, 1000)
		})

		Convey("允许覆盖容量与历史窗口", func() {
			t.Setenv(anonymousSessionTTLEnv, "2h")
			t.Setenv(anonymousSessionMessageLimitEnv, "8")
			t.Setenv(historyMessageLimitEnv, "12")
			t.Setenv(contextTokenBudgetEnv, "500")
			t.Setenv(summaryMaxTokensEnv, "80")
			t.Setenv(summaryRecentTurnsEnv, "3")
			t.Setenv(summaryScanMessageLimitEnv, "100")
			config, err := ConfigFromEnv()

			So(err, ShouldBeNil)
			So(config.AnonymousTTL, ShouldEqual, 2*time.Hour)
			So(config.AnonymousMessageLimit, ShouldEqual, 8)
			So(config.HistoryMessageLimit, ShouldEqual, 12)
			So(config.ContextTokenBudget, ShouldEqual, 500)
			So(config.SummaryMaxTokens, ShouldEqual, 80)
			So(config.SummaryRecentTurns, ShouldEqual, 3)
			So(config.SummaryScanMaxMessages, ShouldEqual, 100)
		})

		Convey("非法配置应在服务启动前失败", func() {
			t.Setenv(anonymousSessionTTLEnv, "zero")
			_, err := ConfigFromEnv()

			So(err, ShouldNotBeNil)
		})
	})
}
