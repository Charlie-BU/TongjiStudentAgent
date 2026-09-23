package skills

import (
	"fmt"
	"sort"
	"strings"

	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
)

// Manifest 是可安全注入 System Prompt 的 Skill 摘要；完整手册仍只由
// system.load_skill 按需返回。
type Manifest struct {
	ID          string
	Description string
}

var manifests = map[string]Manifest{
	skillallowlist.DocGeneratorSkill: {
		ID:          skillallowlist.DocGeneratorSkill,
		Description: "生成、补写或优化中文 Markdown 文档，保持事实准确和作者语气。触发于用户要求成文、局部改写、润色或在不改原意前提下优化文档时。",
	},
	skillallowlist.WebToolsSkill: {
		ID:          skillallowlist.WebToolsSkill,
		Description: "用于必要公开信息补证、明确联网或公开 URL 阅读请求。首问已有证据足够时不加载；同一问题第二次及以上追问或不满意时，场景适合公开检索且未限制联网，默认优先加载并调用网页工具交叉核验或补充视角，不要求先证明已有证据不足。纯表达调整、个人或内部私有事实除外。",
	},
	skillallowlist.CourseAndTeacherGuideSkill: {
		ID:          skillallowlist.CourseAndTeacherGuideSkill,
		Description: "查询同济课程信息、学分、开课学期、任课教师、课程评价，或查看、比较、推荐同济老师及选课时可按需加载本 Skill；tongji.course.* 可直接调用，无需加载任何 Skill。编排课程搜索、详情、开课记录、课评、关联课程、历史教师评价与 AI 总结，聚合可追溯证据，并判断是否加载 web-tools 补充课程政策、最新信息及互联网评价。",
	},
	skillallowlist.LuckinCoffeeSkill: {
		ID:          skillallowlist.LuckinCoffeeSkill,
		Description: "处理瑞幸登录、验证码、查店选品、自取下单、查单取餐或取消订单时必须先加载 luckin-coffee，继续已有流程也适用。任何 luckin.* 调用前遵循本 Skill：先 check，true 才进入业务；无错误且 valid=false 才询问手机号、发送验证码、询问验证码并登录，成功后再次 check。检查报错按提示处理，不触发短信登录。login 与 check 必须串行，不跳过订单确认与预览，不盲目重试下单。纯饮品知识问答不触发。",
	},
	skillallowlist.TongjiCampusToolsSkill: {
		ID:          skillallowlist.TongjiCampusToolsSkill,
		Description: "调用同济校园工具（tongji.course.* 公开课程评价工具除外，可直接调用且无需加载任何 Skill）前必须加载；依据本轮用户身份选择适用工具，覆盖校园生活、资助、考试、课表、成绩、科研和培养计划；覆盖个人校园数据、教师业务、本科与研究生成绩及学业信息。禁止跨身份调用 tools。",
	},
}

// Catalog 返回所有且仅有已批准 Skill 的元数据。它不暴露 Skill 路径或完整手册。
func Catalog() (string, error) {
	skillIDs := append([]string(nil), skillallowlist.Skills()...)
	sort.Strings(skillIDs)
	if len(skillIDs) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("# Available Skills\n")
	for _, skillID := range skillIDs {
		manifest, ok := manifests[skillID]
		if !ok {
			return "", fmt.Errorf("approved skill %q has no manifest", skillID)
		}
		if manifest.ID != skillID || strings.TrimSpace(manifest.Description) == "" {
			return "", fmt.Errorf("invalid manifest for approved skill %q", skillID)
		}
		if _, err := Load(skillID); err != nil {
			return "", fmt.Errorf("validate approved skill %q: %w", skillID, err)
		}

		fmt.Fprintf(&builder, "- `%s`：%s\n", manifest.ID, manifest.Description)
	}
	builder.WriteString("仅在满足触发条件时调用 `system.load_skill` 加载对应 Skill；不要猜测或访问未列出的 Skill。")
	return builder.String(), nil
}
