package webtool

import (
	"context"
	"errors"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
)

func TestWebBoundaries(t *testing.T) {
	Convey("网页边界校验", t, func() {
		for _, raw := range []string{"null", "[]", "{} {}", `{"unknown":true}`, "{", strings.Repeat(" ", 16385)} {
			var v struct{}
			So(Decode(raw, &v), ShouldNotBeNil)
		}
		var v struct{}
		So(Decode("{}", &v), ShouldBeNil)
		text, truncated := Truncate("中文字符", 2)
		So(text, ShouldEqual, "中文")
		So(truncated, ShouldBeTrue)
		So(ValidText("", 5), ShouldBeFalse)
		So(ValidText("中文", 2), ShouldBeTrue)
		for _, raw := range []string{"file:///etc/passwd", "ftp://example.org", "https://localhost", "http://127.0.0.1", "http://[::ffff:127.0.0.1]", "http://169.254.169.254", "https://10.1.2.3", "https://example.internal", "https://x.local", "https://user:pass@example.org", "https://example.org?access_token=x", "https://example.org?code=x", "https://example.org?X-Amz-Signature=x", "https://example.org:8080", "https://2130706433", "http://127.1", "https://example.org?x=%zz"} {
			_, err := PublicURL(raw)
			So(err, ShouldNotBeNil)
		}
		address, err := PublicURL("https://EXAMPLE.org/a?q=公开#part")
		So(err, ShouldBeNil)
		So(address, ShouldEqual, "https://example.org/a?q=公开")
		domains, err := Domains([]string{"Example.org", "example.org."})
		So(err, ShouldBeNil)
		So(domains, ShouldResemble, []string{"example.org"})
		for _, items := range [][]string{{"https://example.org"}, {"127.0.0.1"}, {"example.org/path"}, make([]string, 11)} {
			_, err := Domains(items)
			So(err, ShouldNotBeNil)
		}
		for _, status := range []string{"invalid_arguments", "url_not_allowed", "tool_not_allowed", "rate_limited", "quota_exceeded", "timeout", "fetch_failed", "no_results", "web_unavailable"} {
			result, err := Failure(status)
			So(err, ShouldBeNil)
			So(result, ShouldContainSubstring, status)
		}
		result, err := InvocationError(context.Background(), errors.New("secret"))
		So(err, ShouldBeNil)
		So(result, ShouldNotContainSubstring, "secret")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = InvocationError(ctx, errors.New("secret"))
		So(err, ShouldEqual, context.Canceled)
	})
}
