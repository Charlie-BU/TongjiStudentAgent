package session

import (
	"errors"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPostgresStoreValidation(t *testing.T) {
	Convey("认证 PostgreSQL 会话输入", t, func() {
		Convey("会话归属与消息约束应在访问数据库前校验", func() {
			So(errors.Is(validateOwnerAndSessionID("", "user-001"), ErrInvalidSessionID), ShouldBeTrue)
			So(errors.Is(validateOwnerAndSessionID("ses-001", ""), ErrInvalidOwner), ShouldBeTrue)

			_, err := validateMessage(NewMessage{Role: MessageRoleUser, Content: " "})
			So(errors.Is(err, ErrInvalidMessage), ShouldBeTrue)
			message, err := validateMessage(NewMessage{Role: MessageRoleAssistant, Content: " 你好 "})
			So(err, ShouldBeNil)
			So(message.Content, ShouldEqual, "你好")
		})

		Convey("建表 DDL 应包含消息顺序约束", func() {
			schema := strings.Join(postgresSchemaStatements, "\n")
			So(strings.Contains(schema, "UNIQUE (session_id, sequence)"), ShouldBeTrue)
		})
	})
}
