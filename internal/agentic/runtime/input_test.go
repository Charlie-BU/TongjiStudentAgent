package runtime

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildInputMessages(t *testing.T) {
	Convey("构建 Agent 的本轮输入消息", t, func() {
		messages, err := buildInputMessages("查询 <课程> & 成绩", "# Available Skills\n- `doc-generator`: 文档处理", time.Date(2026, time.July, 2, 15, 10, 18, 0, reminderLocation))

		Convey("应先注入独立的系统提醒，再注入 XML 包裹的用户请求", func() {
			So(err, ShouldBeNil)
			So(messages, ShouldHaveLength, 2)
			So(messages[0].Role, ShouldEqual, schema.User)
			So(messages[0].Content, ShouldEqual, "<system-reminder>\n当前日期：2026-07-02 周四 15:10:18（Etc/GMT-8）\n\n# Available Skills\n- `doc-generator`: 文档处理\n</system-reminder>")
			So(messages[1].Role, ShouldEqual, schema.User)
			So(messages[1].Content, ShouldEqual, "<interaction_request>\n  <user_query>查询 &lt;课程&gt; &amp; 成绩</user_query>\n</interaction_request>")
		})
	})
}
