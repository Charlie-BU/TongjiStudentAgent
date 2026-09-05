package systemtools

import (
	"context"
	"testing"

	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	loadskill "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/load_skill"
	managetaskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/manage_task_plan"
	searchknowledge "github.com/Charlie-BU/TongjiStudent/internal/agentic/systemtools/search_knowledge"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/knowledge"
	"github.com/Charlie-BU/TongjiStudent/internal/integration/tavily"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSystemToolsRequireToolAllowlist(t *testing.T) {
	Convey("静态系统工具注册", t, func() {
		Convey("未加白工具不应注册", func() {
			So(buildTools(func(string) bool { return false }, nil, nil, &tavily.Client{}), ShouldBeEmpty)
		})

		Convey("已加白工具应注册", func() {
			tools := Tools()

			So(tools, ShouldHaveLength, 1)
			info, err := tools[0].Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, loadskill.LoadSkillToolName)
		})
		Convey("网页能力需要客户端且逐项加白", func() {
			So(Tools(WithTavilyClient(nil)), ShouldHaveLength, 1)
			tools := buildTools(func(name string) bool { return name == toolallowlist.URLFetchTool }, nil, nil, &tavily.Client{})
			So(tools, ShouldHaveLength, 1)
			info, err := tools[0].Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, toolallowlist.URLFetchTool)
		})

		Convey("注入任务计划 repository 后应注册管理工具", func() {
			tools := Tools(WithTaskPlanRepository(&fakeTaskPlanRepository{}))

			So(tools, ShouldHaveLength, 2)
			info, err := tools[1].Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, managetaskplan.ManageTaskPlanToolName)
		})

		Convey("仅在知识库客户端已启用时注册检索工具", func() {
			tools := Tools(WithKnowledgeClient(&knowledge.Client{}))

			So(tools, ShouldHaveLength, 2)
			info, err := tools[1].Info(context.Background())
			So(err, ShouldBeNil)
			So(info.Name, ShouldEqual, searchknowledge.SearchKnowledgeToolName)
		})
	})
}

type fakeTaskPlanRepository struct{}

func (*fakeTaskPlanRepository) GetTaskPlan(context.Context) (*taskplan.TaskPlan, error) {
	return nil, nil
}
func (*fakeTaskPlanRepository) SaveTaskPlan(context.Context, int64, []taskplan.TaskItem) (taskplan.TaskPlan, error) {
	return taskplan.TaskPlan{}, nil
}
func (*fakeTaskPlanRepository) ClearTaskPlan(context.Context, int64) error { return nil }
