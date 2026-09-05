package chat

import (
	"context"
	"errors"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	"testing"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	agenticsession "github.com/Charlie-BU/TongjiStudent/internal/agentic/session"
	sessioncontext "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/context"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	"github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
	"github.com/cloudwego/eino/schema"
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

func TestRunFailedData(t *testing.T) {
	Convey("Run 失败事件包含错误原因和 HTTP 状态码", t, func() {
		data := runFailedData("agent_execution_failed", "Agent 执行失败", errors.New("model request failed: status code: 429"))

		So(data.Code, ShouldEqual, "agent_execution_failed")
		So(data.Message, ShouldEqual, "Agent 执行失败")
		So(data.Reason, ShouldEqual, "model request failed: status code: 429")
		So(data.StatusCode, ShouldEqual, 429)
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
		Convey("启用网页客户端后注册两个公开工具且不影响 MCP 名单", func() {
			registered := systemtools.Tools(systemtools.WithTavilyClient(&tavily.Client{}))
			So(registered, ShouldHaveLength, 3)
			for index, name := range []string{toolallowlist.WebSearchTool, toolallowlist.URLFetchTool} {
				info, err := registered[index+1].Info(context.Background())
				So(err, ShouldBeNil)
				So(info.Name, ShouldEqual, name)
				So(toolallowlist.MCPTools(), ShouldNotContain, name)
			}
		})

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
			taskPlanRepository:    &recordingTaskPlanRepository{},
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
		So(runner.hasTaskPlanScope, ShouldBeTrue)
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

func TestWaitForSessionTurn(t *testing.T) {
	Convey("等待会话轮次", t, func() {
		locker := &deleteWaitTurnLocker{blocked: make(chan struct{}), allowAcquire: make(chan struct{})}
		service := &Service{turnLocker: locker}
		done := make(chan agenticsession.TurnRelease, 1)
		errs := make(chan error, 1)

		go func() {
			release, err := service.waitForSessionTurn(context.Background(), "ses-001")
			done <- release
			errs <- err
		}()

		<-locker.blocked
		select {
		case <-done:
			t.Fatal("当前轮次结束前不应获取删除锁")
		default:
		}
		close(locker.allowAcquire)

		So(<-errs, ShouldBeNil)
		So(<-done, ShouldNotBeNil)
	})
}

func TestAppendAgentMessagePersistsArkResponseCache(t *testing.T) {
	Convey("Agent 输出的 Ark response-chain 元数据", t, func() {
		operations := make([]string, 0, 1)
		store := &recordingEphemeralStore{operations: &operations}
		service := &Service{ephemeralSessionStore: store}
		message := schema.AssistantMessage("查询完成", nil)
		message.Extra = map[string]any{
			"ark-response-id":              "resp-001",
			"ark-response-cache-expire-at": int64(1_785_000_000),
		}

		err := service.appendAgentMessage(context.Background(), "anon-001", "run-001", message)

		Convey("应持久化并在下一轮上下文恢复 SDK 可识别字段", func() {
			So(err, ShouldBeNil)
			So(store.appended, ShouldHaveLength, 1)
			So(store.appended[0].ResponseID, ShouldEqual, "resp-001")
			So(store.appended[0].ResponseCacheExpiresAt, ShouldEqual, int64(1_785_000_000))

			history := agenticsession.Message{
				Sequence:               1,
				Role:                   store.appended[0].Role,
				Content:                store.appended[0].Content,
				ResponseID:             store.appended[0].ResponseID,
				ResponseCacheExpiresAt: store.appended[0].ResponseCacheExpiresAt,
			}
			messages, assembleErr := sessioncontext.NewContextAssembler().AssembleForTurn(context.Background(), sessioncontext.TurnInput{
				DynamicReminder: schema.UserMessage("<system-reminder>当前日期</system-reminder>"),
				History:         []agenticsession.Message{history},
				UserMessage:     schema.UserMessage("继续查询"),
			})
			So(assembleErr, ShouldBeNil)
			So(messages[1].Extra["ark-response-id"], ShouldEqual, "resp-001")
			So(messages[1].Extra["ark-response-cache-expire-at"], ShouldEqual, int64(1_785_000_000))
		})
	})
}

type recordingEphemeralStore struct {
	operations *[]string
	history    []agenticsession.Message
	appended   []agenticsession.NewMessage
}

func (s *recordingEphemeralStore) Create(context.Context) (agenticsession.Session, error) {
	return agenticsession.Session{}, nil
}

func (s *recordingEphemeralStore) Get(_ context.Context, sessionID string) (agenticsession.Session, error) {
	return agenticsession.Session{ID: sessionID, Persistence: agenticsession.PersistenceEphemeral}, nil
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
	operations       *[]string
	query            string
	history          []agenticsession.Message
	response         string
	hasTaskPlanScope bool
}

type recordingTaskPlanRepository struct {
	plan *taskplan.TaskPlan
}

func (r *recordingTaskPlanRepository) GetTaskPlan(context.Context) (*taskplan.TaskPlan, error) {
	return r.plan, nil
}

func (r *recordingTaskPlanRepository) SaveTaskPlan(context.Context, int64, []taskplan.TaskItem) (taskplan.TaskPlan, error) {
	return taskplan.TaskPlan{}, errors.New("SaveTaskPlan should not be called")
}

func (r *recordingTaskPlanRepository) ClearTaskPlan(context.Context, int64) error {
	return errors.New("ClearTaskPlan should not be called")
}

func (r *recordingSessionRuntime) StreamWithHistoryAndMessages(ctx context.Context, query, _ string, history []agenticsession.Message, _ func(agentevent.Event), record func(context.Context, *schema.Message) error) (string, error) {
	r.query = query
	r.history = history
	_, r.hasTaskPlanScope = taskplan.TaskPlanScopeFromContext(ctx)
	*r.operations = append(*r.operations, "runtime")
	if err := record(ctx, schema.AssistantMessage(r.response, nil)); err != nil {
		return "", err
	}
	return r.response, nil
}

type noOpTurnLocker struct{}

func (noOpTurnLocker) AcquireTurn(context.Context, string) (agenticsession.TurnRelease, error) {
	return func() {}, nil
}

type deleteWaitTurnLocker struct {
	blocked      chan struct{}
	allowAcquire chan struct{}
}

func (l *deleteWaitTurnLocker) AcquireTurn(context.Context, string) (agenticsession.TurnRelease, error) {
	select {
	case <-l.allowAcquire:
		return func() {}, nil
	default:
	}
	select {
	case <-l.blocked:
	default:
		close(l.blocked)
	}
	return nil, agenticsession.ErrTurnInProgress
}

func eventTypes(events []agentevent.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
