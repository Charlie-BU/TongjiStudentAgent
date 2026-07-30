package skills

import (
	"context"
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRunStateLoadOnce(t *testing.T) {
	Convey("同一 Run 内加载 Skill", t, func() {
		state := NewRunState()
		calls := 0
		loader := func() (string, error) {
			calls++
			return "skill manual", nil
		}

		Convey("首次加载返回手册并记录状态", func() {
			content, alreadyLoaded, err := state.LoadOnce("doc-generator", loader)

			So(err, ShouldBeNil)
			So(alreadyLoaded, ShouldBeFalse)
			So(content, ShouldEqual, "skill manual")
			So(calls, ShouldEqual, 1)
		})

		Convey("重复加载不会再次执行 loader 或返回手册", func() {
			_, _, err := state.LoadOnce("doc-generator", loader)
			content, alreadyLoaded, err := state.LoadOnce("doc-generator", loader)

			So(err, ShouldBeNil)
			So(alreadyLoaded, ShouldBeTrue)
			So(content, ShouldBeBlank)
			So(calls, ShouldEqual, 1)
		})

		Convey("失败不会记录，后续可以重试", func() {
			_, alreadyLoaded, err := state.LoadOnce("doc-generator", func() (string, error) {
				calls++
				return "", errors.New("unavailable")
			})
			content, retriedAlreadyLoaded, retryErr := state.LoadOnce("doc-generator", loader)

			So(err, ShouldNotBeNil)
			So(alreadyLoaded, ShouldBeFalse)
			So(retryErr, ShouldBeNil)
			So(retriedAlreadyLoaded, ShouldBeFalse)
			So(content, ShouldEqual, "skill manual")
			So(calls, ShouldEqual, 2)
		})
	})
}

func TestRunStateContext(t *testing.T) {
	Convey("Run State Context", t, func() {
		state := NewRunState()
		loaded, ok := RunStateFromContext(WithRunState(context.Background(), state))

		So(ok, ShouldBeTrue)
		So(loaded, ShouldEqual, state)
	})
}
