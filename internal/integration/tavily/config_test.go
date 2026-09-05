package tavily

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNewFromEnv(t *testing.T) {
	Convey("网页能力配置", t, func() {
		t.Setenv("TAVILY_ENABLED", "")
		t.Setenv("TAVILY_API_KEY", "")
		client, err := NewFromEnv()
		So(err, ShouldBeNil)
		So(client, ShouldBeNil)
		t.Setenv("TAVILY_ENABLED", "invalid")
		_, err = NewFromEnv()
		So(err, ShouldNotBeNil)
		t.Setenv("TAVILY_ENABLED", "true")
		_, err = NewFromEnv()
		So(err, ShouldNotBeNil)
		t.Setenv("TAVILY_API_KEY", "test-key")
		client, err = NewFromEnv()
		So(err, ShouldBeNil)
		So(client.http.Timeout, ShouldEqual, 30*time.Second)
		for _, value := range []time.Duration{0, -time.Second, 121 * time.Second} {
			_, err = NewClient("test-key", value, nil)
			So(err, ShouldNotBeNil)
		}
		t.Setenv("TAVILY_ENABLED", "false")
		client, err = NewFromEnv()
		So(err, ShouldBeNil)
		So(client, ShouldBeNil)
	})
}
