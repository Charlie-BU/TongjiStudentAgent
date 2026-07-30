package chat

import (
	"context"
	"errors"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatRequiresInitializedDefaultService(t *testing.T) {
	Convey("使用默认聊天服务", t, func() {
		original := defaultService
		defaultService = nil
		t.Cleanup(func() { defaultService = original })

		Convey("默认服务未初始化", func() {
			response, err := Chat(context.Background(), "你好")

			Convey("应返回初始化错误且不产生响应", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat service is not initialized")
			})
		})
	})
}

func TestServiceLoadUserInfo(t *testing.T) {
	Convey("加载个人基础信息", t, func() {
		Convey("请求未携带 access token", func() {
			called := false
			service := &Service{studentInfoLoader: func(context.Context, string) (*tongjiapi.StudentInfo, error) {
				called = true
				return nil, nil
			}}

			info, err := service.loadFormattedStudentInfo(context.Background())

			So(err, ShouldBeNil)
			So(info, ShouldBeBlank)
			So(called, ShouldBeFalse)
		})

		Convey("请求携带 access token", func() {
			service := &Service{studentInfoLoader: func(_ context.Context, accessToken string) (*tongjiapi.StudentInfo, error) {
				So(accessToken, ShouldEqual, "test-access-token")
				return &tongjiapi.StudentInfo{
					Name:          "测试同学",
					TrainingLevel: "本科",
					CurrentGrade:  2023,
					Faculty:       "计算机科学与技术学院",
					LeaveSchool:   "校内在读",
				}, nil
			}}

			info, err := service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-access-token"))

			So(err, ShouldBeNil)
			So(info, ShouldEqual, "当前年级：2023\n学院：计算机科学与技术学院\n在校状态：校内在读\n姓名：测试同学\n培养层次：本科")
		})

		Convey("上游获取失败", func() {
			service := &Service{studentInfoLoader: func(context.Context, string) (*tongjiapi.StudentInfo, error) {
				return nil, errors.New("upstream unavailable")
			}}

			_, err := service.loadFormattedStudentInfo(platformauth.WithAccessToken(context.Background(), "test-access-token"))

			So(err, ShouldNotBeNil)
		})
	})
}

func TestFormatStudentInfo(t *testing.T) {
	Convey("裁剪个人基础信息", t, func() {
		info := tongjiapi.FormatStudentInfo(&tongjiapi.StudentInfo{
			Birthday:               "2004-12-09 00:00:00",
			ChinaSon:               "非港澳台",
			CultureProfession:      "软件工程(42014）",
			CurrentGrade:           2023,
			EnrolDate:              "2023-09-01 00:00:00",
			EnrolMethods:           "一般统考",
			EnrolSeason:            "秋季",
			ExpectedGraduationDate: "2027-07-01 00:00:00",
			Faculty:                "计算机科学与技术学院",
			FormLearning:           "普通全日制",
			HouseholdRegister:      "内蒙古自治区",
			IsDobleDegree:          "否",
			IsOverseas:             "否",
			LeaveSchool:            "校内在读",
			LengthSchooling:        "4",
			MailingAddress:         "测试家庭地址",
			MajorDirection:         "嵌入式软件与系统",
			Name:                   "测试同学",
			NameSpelling:           "TEST TONGXUE",
			Nation:                 "汉族",
			PoliticalStatus:        "群众",
			Sex:                    "男",
			SpcialPlan:             "无专项计划",
			State:                  "中国",
			StudentID:              "2350939",
			TrainingCategory:       "学历学位生",
			TrainingLevel:          "本科",
		})

		Convey("应以字段含义作为键名返回指定资料", func() {
			So(info, ShouldContainSubstring, "姓名：测试同学")
			So(info, ShouldContainSubstring, "生日：2004-12-09 00:00:00")
			So(info, ShouldContainSubstring, "家庭地址：测试家庭地址")
			So(info, ShouldContainSubstring, "学（工）号：2350939")
			So(info, ShouldContainSubstring, "专业方向：嵌入式软件与系统")
			So(info, ShouldNotContainSubstring, "mailingAddress")
			So(info, ShouldNotContainSubstring, "studentId")
		})
	})
}

func TestMCPToolAllowlist(t *testing.T) {
	Convey("聊天服务的远程 MCP Tool 白名单", t, func() {
		tools := toolallowlist.MCPTools()

		Convey("只注册维护在 allowlist 中的远程工具", func() {
			So(tools, ShouldResemble, []string{toolallowlist.TongjiStudentScoreTool})
		})

		Convey("调用方修改返回值不应影响后续服务初始化", func() {
			tools[0] = "untrusted-tool"

			So(toolallowlist.MCPTools(), ShouldResemble, []string{toolallowlist.TongjiStudentScoreTool})
		})
	})
}

func TestAgentToolsIncludesAllowedStaticTools(t *testing.T) {
	Convey("聊天服务的静态系统 Tool 注册", t, func() {
		tools := systemtools.Tools()

		Convey("应注入已加白的静态系统工具", func() {
			So(tools, ShouldHaveLength, 1)
			info, err := tools[0].Info(context.Background())

			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, loadskill.LoadSkillToolName)
		})
	})
}

func TestLoadSystemInstructionDoesNotContainSkillCatalog(t *testing.T) {
	Convey("未启用 CozeLoop 时的 System Prompt", t, func() {
		t.Setenv("COZELOOP_ENABLED", "false")
		instruction, err := loadSystemInstruction(context.Background())

		Convey("应保持为空，Skill Catalog 由独立 User 提醒消息承载", func() {
			So(err, ShouldBeNil)
			So(instruction, ShouldBeBlank)
		})
	})
}

func TestServiceChatRequiresRuntime(t *testing.T) {
	Convey("通过聊天服务执行对话", t, func() {
		Convey("Runtime 未初始化", func() {
			service := &Service{}
			var events []agentevent.Event
			response, err := service.Stream(context.Background(), "你好", func(event agentevent.Event) {
				events = append(events, event)
			})

			Convey("应返回初始化错误且以失败终态收尾", func() {
				So(response, ShouldBeBlank)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "chat service is not initialized")
				So(events, ShouldHaveLength, 2)
				So(events[0].Type, ShouldEqual, agentevent.RunStarted)
				So(events[1].Type, ShouldEqual, agentevent.RunFailed)
			})
		})
	})
}
