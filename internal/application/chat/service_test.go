package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	. "github.com/smartystreets/goconvey/convey"
)

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
		expectedTools := []string{
			toolallowlist.TongjiAnnualBillTool,
			toolallowlist.TongjiCardSpendingFlowTool,
			toolallowlist.TongjiStudentTimetableTool,
			toolallowlist.TongjiStudentDetailedInfoTool,
			toolallowlist.TongjiStudentScoreTool,
			toolallowlist.TongjiTermCalendarTool,
			toolallowlist.TongjiCurrentTermCalendarTool,
			toolallowlist.TongjiCETScoreTool,
			toolallowlist.TongjiBookLendInfoTool,
			toolallowlist.TongjiStatisticsInfoTool,
			toolallowlist.TongjiStipendInfoTool,
			toolallowlist.TongjiAccommodationInfoTool,
			toolallowlist.TongjiCompetitionPrizeTool,
			toolallowlist.TongjiHonoraryTitleTool,
			toolallowlist.TongjiScholarshipInfoTool,
			toolallowlist.TongjiSchoolAccessTool,
			toolallowlist.TongjiLibraryAccessTool,
			toolallowlist.TongjiUserBasicInfoTool,
			toolallowlist.TongjiCourseDetailTool,
			toolallowlist.TongjiCourseRelatedTool,
			toolallowlist.TongjiFindMajorByGradeTool,
			toolallowlist.TongjiCourseCatalogTool,
			toolallowlist.TongjiCalendarListTool,
			toolallowlist.TongjiGradeListTool,
		}

		Convey("只注册维护在 allowlist 中的远程工具", func() {
			So(tools, ShouldResemble, expectedTools)
		})

		Convey("调用方修改返回值不应影响后续服务初始化", func() {
			tools[0] = "untrusted-tool"

			So(toolallowlist.MCPTools(), ShouldResemble, expectedTools)
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

func TestStreamSessionEndToEnd(t *testing.T) {
	Convey("会话消息主链路", t, func() {
		operations := make([]string, 0, 4)
		store := &recordingEphemeralStore{
			operations: &operations,
			history: []agenticsession.Message{
				{ID: "msg-001", SessionID: "anon-001", Sequence: 1, Role: agenticsession.MessageRoleUser, Content: "上一轮问题"},
				{ID: "msg-002", SessionID: "anon-001", Sequence: 2, Role: agenticsession.MessageRoleAssistant, Content: "上一轮回答"},
			},
		}
		runner := &recordingSessionRuntime{operations: &operations, response: "本轮回答"}
		service := &Service{
			runtime:               runner,
			ephemeralSessionStore: store,
			turnLocker:            noOpTurnLocker{},
			historyMessageLimit:   20,
		}
		events := make([]agentevent.Event, 0, 4)

		response, err := service.StreamSession(context.Background(), "anon-001", "本轮问题", func(event agentevent.Event) {
			events = append(events, event)
		})

		So(err, ShouldBeNil)
		So(response, ShouldEqual, "本轮回答")
		So(runner.query, ShouldEqual, "本轮问题")
		So(runner.history, ShouldResemble, store.history)
		So(store.appended, ShouldHaveLength, 2)
		So(store.appended[0].Role, ShouldEqual, agenticsession.MessageRoleUser)
		So(store.appended[0].Content, ShouldEqual, "本轮问题")
		So(store.appended[0].RunID, ShouldNotBeBlank)
		So(store.appended[1].Role, ShouldEqual, agenticsession.MessageRoleAssistant)
		So(store.appended[1].Content, ShouldEqual, "本轮回答")
		So(store.appended[1].RunID, ShouldEqual, store.appended[0].RunID)
		So(operations, ShouldResemble, []string{"list", "append:user", "runtime", "append:assistant"})
		So(eventTypes(events), ShouldResemble, []string{
			agentevent.RunStarted,
			agentevent.AgentStatus,
			agentevent.AgentStatus,
			agentevent.RunCompleted,
		})
		So(events[0].RunID, ShouldEqual, store.appended[0].RunID)
	})
}

func TestCompactMemoryByCompleteRun(t *testing.T) {
	Convey("动态摘要只压缩完整的旧 run", t, func() {
		summarizer := &recordingSummarizer{}
		service := &Service{summarizer: summarizer, contextTokenBudget: 60, summaryMaxTokens: 20, summaryRecentTurns: 2}
		messages := []agenticsession.Message{
			{Sequence: 1, RunID: "run-001", Role: agenticsession.MessageRoleUser, Content: strings.Repeat("甲", 100)},
			{Sequence: 2, RunID: "run-001", Role: agenticsession.MessageRoleAssistant, Content: "旧回答"},
			{Sequence: 3, RunID: "run-002", Role: agenticsession.MessageRoleUser, Content: strings.Repeat("乙", 100)},
			{Sequence: 4, RunID: "run-002", Role: agenticsession.MessageRoleAssistant, Content: "中间回答"},
			{Sequence: 5, RunID: "run-003", Role: agenticsession.MessageRoleUser, Content: strings.Repeat("丙", 100)},
			{Sequence: 6, RunID: "run-003", Role: agenticsession.MessageRoleAssistant, Content: "最近回答"},
		}

		snapshot, remaining, err := service.compactMemory(context.Background(), agenticsession.MemorySnapshot{}, messages, "本轮问题")

		So(err, ShouldBeNil)
		So(snapshot.Summary, ShouldEqual, "摘要-1")
		So(snapshot.AnchorSequence, ShouldEqual, int64(2))
		So(summarizer.calls, ShouldHaveLength, 1)
		So(summarizer.calls[0][0].RunID, ShouldEqual, "run-001")
		So(remaining, ShouldHaveLength, 4)
		So(remaining[0].RunID, ShouldEqual, "run-002")
		So(remaining[3].RunID, ShouldEqual, "run-003")
	})
}

type recordingSummarizer struct {
	calls [][]agenticsession.Message
}

func (s *recordingSummarizer) Summarize(_ context.Context, _ string, turns []agenticsession.Message, _ int) (string, error) {
	s.calls = append(s.calls, turns)
	return fmt.Sprintf("摘要-%d", len(s.calls)), nil
}

type recordingEphemeralStore struct {
	operations *[]string
	history    []agenticsession.Message
	appended   []agenticsession.NewMessage
}

func (s *recordingEphemeralStore) Create(context.Context) (agenticsession.Session, error) {
	return agenticsession.Session{}, nil
}

func (s *recordingEphemeralStore) Get(context.Context, string) (agenticsession.Session, error) {
	return agenticsession.Session{}, nil
}

func (s *recordingEphemeralStore) Append(_ context.Context, _ string, message agenticsession.NewMessage) (agenticsession.AppendResult, error) {
	s.appended = append(s.appended, message)
	*s.operations = append(*s.operations, "append:"+string(message.Role))
	return agenticsession.AppendResult{Created: true}, nil
}

func (s *recordingEphemeralStore) ListMessages(context.Context, string, int) ([]agenticsession.Message, error) {
	*s.operations = append(*s.operations, "list")
	return s.history, nil
}

type recordingSessionRuntime struct {
	operations *[]string
	query      string
	history    []agenticsession.Message
	response   string
}

func (r *recordingSessionRuntime) StreamWithHistory(_ context.Context, query, _ string, history []agenticsession.Message, _ func(agentevent.Event)) (string, error) {
	r.query = query
	r.history = history
	*r.operations = append(*r.operations, "runtime")
	return r.response, nil
}

type noOpTurnLocker struct{}

func (noOpTurnLocker) AcquireTurn(context.Context, string) (agenticsession.TurnRelease, error) {
	return func() {}, nil
}

func eventTypes(events []agentevent.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
