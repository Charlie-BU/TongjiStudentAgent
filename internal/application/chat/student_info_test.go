package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadFormattedStudentInfo(t *testing.T) {
	Convey("通过复用的同济客户端加载学生信息", t, func() {
		calls := 0
		var tokens []string
		status := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			tokens = append(tokens, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			fmt.Fprint(w, `{"code":"A00000","msg":"ok","data":[{"currentGrade":2023,"faculty":"测试学院","leaveSchool":"校内在读","name":"测试同学","trainingLevel":"本科"}]}`)
		}))
		defer server.Close()
		client, err := tongjiapi.New(tongjiapi.Config{ClientID: "test", ClientSecret: "test", RedirectURI: "https://example.test/callback", StateSecret: "test", AuthorizationEndpoint: server.URL, TokenEndpoint: server.URL, APIBaseURL: server.URL})
		So(err, ShouldBeNil)
		service := &Service{tongjiClient: client}
		info, err := service.loadFormattedStudentInfo(context.Background())
		So(err, ShouldBeNil)
		So(info, ShouldBeBlank)
		So(calls, ShouldEqual, 0)

		for _, token := range []string{"test-token-a", "test-token-b"} {
			info, err = service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), token))
			So(err, ShouldBeNil)
			So(info, ShouldEqual, "当前年级：2023\n学院：测试学院\n在校状态：校内在读\n姓名：测试同学\n培养层次：本科")
			So(service.tongjiClient, ShouldEqual, client)
		}
		So(tokens, ShouldResemble, []string{"Bearer test-token-a", "Bearer test-token-b"})
		status = http.StatusServiceUnavailable
		_, err = service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-token"))
		So(err, ShouldNotBeNil)
	})
	Convey("携带 token 时不能因客户端缺失静默跳过", t, func() {
		service := &Service{}
		_, err := service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-token"))
		So(err, ShouldNotBeNil)
		info, err := service.loadFormattedStudentInfo(context.Background())
		So(err, ShouldBeNil)
		So(info, ShouldBeBlank)
	})
}
