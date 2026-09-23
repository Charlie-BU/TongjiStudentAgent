// Package tool 集中维护允许调用的 tool 白名单。
package tool

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// 静态系统 Tool
	LoadSkillTool       = "system.load_skill"
	ManageTaskPlanTool  = "system.manage_task_plan"
	SearchKnowledgeTool = "system.search_knowledge"
	WebSearchTool       = "system.web_search"
	URLFetchTool        = "system.url_fetch"
	// 远程 MCP Tool
	TongjiUserAnnualBillTool              = "tongji.user.annual_bill"
	TongjiUserCardSpendingFlowTool        = "tongji.user.card_spending_flow"
	TongjiStudentTimetableTool            = "tongji.student.timetable"
	TongjiStudentDetailedInfoTool         = "tongji.student.detailed_info"
	TongjiBachelorScoreTool               = "tongji.bachelor.score"
	TongjiUserTermCalendarTool            = "tongji.user.term-calendar"
	TongjiUserCurrentTermCalendarTool     = "tongji.user.current-term-calendar"
	TongjiCETScoreTool                    = "tongji.student.cet-score"
	TongjiUserBookLendInfoTool            = "tongji.user.book-lend-info"
	TongjiUserStatisticsInfoTool          = "tongji.user.statistics-info"
	TongjiStipendInfoTool                 = "tongji.student.stipend-info"
	TongjiAccommodationInfoTool           = "tongji.student.accommodation-info"
	TongjiBachelorCompetitionPrizeTool    = "tongji.bachelor.competition_prize"
	TongjiHonoraryTitleTool               = "tongji.student.honorary_title"
	TongjiScholarshipInfoTool             = "tongji.student.scholarship_info"
	TongjiUserSchoolAccessTool            = "tongji.user.school_access"
	TongjiUserLibraryAccessTool           = "tongji.user.library_access"
	TongjiPostgraduateGPATool             = "tongji.postgraduate.gpa"
	TongjiPostgraduateRequiredCreditTool  = "tongji.postgraduate.required_credit"
	TongjiUserResearchProjectsTool        = "tongji.user.research_projects"
	TongjiUserResearchWorksTool           = "tongji.user.research_works"
	TongjiUserContactInfoTool             = "tongji.user.contact_info"
	TongjiUserUpdateContactInfoTool       = "tongji.user.update_contact_info"
	TongjiStudentHardshipAllowanceTool    = "tongji.student.hardship_allowance"
	TongjiStudentLoanTool                 = "tongji.student.loan"
	TongjiStudentWorkStudyTool            = "tongji.student.work_study"
	TongjiTeacherTimetableTool            = "tongji.teacher.timetable"
	TongjiUserCardBalanceTool             = "tongji.user.card_balance"
	TongjiPostgraduatePlanProgressTool    = "tongji.postgraduate.plan_progress"
	TongjiPostgraduatePlanTool            = "tongji.postgraduate.plan"
	TongjiPostgraduateMajorsTool          = "tongji.postgraduate.majors"
	TongjiPostgraduateScoreTool           = "tongji.postgraduate.score"
	TongjiUserResearchPatentsTool         = "tongji.user.research_patents"
	TongjiStudentFinalExamsTool           = "tongji.student.final_exams"
	TongjiStudentDeferredExamsTool        = "tongji.student.deferred_exams"
	TongjiBachelorGradeSummaryTool        = "tongji.bachelor.grade_summary"
	TongjiUserEmailTool                   = "tongji.user.email"
	TongjiTeacherTitleTool                = "tongji.teacher.title"
	TongjiStudentCounselorTool            = "tongji.student.counselor"
	TongjiPostgraduateCompletedCreditTool = "tongji.postgraduate.completed_credit"
	TongjiPostgraduateDegreeCreditTool    = "tongji.postgraduate.degree_credit"
	TongjiPostgraduateDegreeAverageTool   = "tongji.postgraduate.degree_average"
	TongjiLegacyTeacherReviewsTool        = "tongji.course.legacy-teacher-reviews"
	TongjiCourseDetailTool                = "tongji.course.course-detail"
	TongjiCourseRelatedTool               = "tongji.course.course-related"
	TongjiCourseReviewsTool               = "tongji.course.reviews"
	TongjiCourseSummaryTool               = "tongji.course.summary"
	TongjiCourseCatalogTool               = "tongji.course.search"
	LuckinAuthCheckTool                   = "luckin.auth.check"
	LuckinAuthSendSMSCodeTool             = "luckin.auth.send_sms_code"
	LuckinAuthLoginTool                   = "luckin.auth.login"
	LuckinShopSearchTool                  = "luckin.shop.search"
	LuckinProductSearchTool               = "luckin.product.search"
	LuckinProductDetailTool               = "luckin.product.detail"
	LuckinProductSwitchTool               = "luckin.product.switch"
	LuckinOrderPreviewTool                = "luckin.order.preview"
	LuckinOrderCreateTool                 = "luckin.order.create"
	LuckinOrderGetTool                    = "luckin.order.get"
	LuckinOrderCancelTool                 = "luckin.order.cancel"
)

var (
	allowedSystemTools = []string{
		LoadSkillTool,
		ManageTaskPlanTool,
		SearchKnowledgeTool,
		WebSearchTool,
		URLFetchTool,
	}

	allowedMCPTools = []string{
		TongjiUserAnnualBillTool,
		TongjiUserCardSpendingFlowTool,
		TongjiStudentTimetableTool,
		TongjiStudentDetailedInfoTool,
		TongjiBachelorScoreTool,
		TongjiUserTermCalendarTool,
		TongjiUserCurrentTermCalendarTool,
		TongjiCETScoreTool,
		TongjiUserBookLendInfoTool,
		TongjiUserStatisticsInfoTool,
		TongjiStipendInfoTool,
		TongjiAccommodationInfoTool,
		TongjiBachelorCompetitionPrizeTool,
		TongjiHonoraryTitleTool,
		TongjiScholarshipInfoTool,
		TongjiUserSchoolAccessTool,
		TongjiUserLibraryAccessTool,
		TongjiPostgraduateGPATool,
		TongjiPostgraduateRequiredCreditTool,
		TongjiUserResearchProjectsTool,
		TongjiUserResearchWorksTool,
		TongjiUserContactInfoTool,
		TongjiUserUpdateContactInfoTool,
		TongjiStudentHardshipAllowanceTool,
		TongjiStudentLoanTool,
		TongjiStudentWorkStudyTool,
		TongjiTeacherTimetableTool,
		TongjiUserCardBalanceTool,
		TongjiPostgraduatePlanProgressTool,
		TongjiPostgraduatePlanTool,
		TongjiPostgraduateMajorsTool,
		TongjiPostgraduateScoreTool,
		TongjiUserResearchPatentsTool,
		TongjiStudentFinalExamsTool,
		TongjiStudentDeferredExamsTool,
		TongjiBachelorGradeSummaryTool,
		TongjiUserEmailTool,
		TongjiTeacherTitleTool,
		TongjiStudentCounselorTool,
		TongjiPostgraduateCompletedCreditTool,
		TongjiPostgraduateDegreeCreditTool,
		TongjiPostgraduateDegreeAverageTool,
		TongjiLegacyTeacherReviewsTool,
		TongjiCourseDetailTool,
		TongjiCourseRelatedTool,
		TongjiCourseReviewsTool,
		TongjiCourseSummaryTool,
		TongjiCourseCatalogTool,

		LuckinAuthCheckTool,
		LuckinAuthSendSMSCodeTool,
		LuckinAuthLoginTool,
		LuckinShopSearchTool,
		LuckinProductSearchTool,
		LuckinProductDetailTool,
		LuckinProductSwitchTool,
		LuckinOrderPreviewTool,
		LuckinOrderCreateTool,
		LuckinOrderGetTool,
		LuckinOrderCancelTool,
	}
)

// SystemTools 返回已批准 Tool 名称的副本，调用方修改结果不会影响 allowlist。
func SystemTools() []string {
	return append([]string(nil), allowedSystemTools...)
}

// MCPTools 返回已批准 Tool 名称的副本，调用方修改结果不会影响 allowlist。
func MCPTools() []string {
	return append([]string(nil), allowedMCPTools...)
}

// IsAllowedTool 判断 Tool 名称是否已被应用 allowlist 明确批准。
func IsAllowedTool(toolName string) bool {
	for _, allowedTool := range append(allowedSystemTools, allowedMCPTools...) {
		if toolName == allowedTool {
			return true
		}
	}
	return false
}

// ValidateToolAllowlist 确保远程 Tool 只能由非空且无重复的 allowlist 注册。
func ValidateToolAllowlist(toolNames []string) error {
	if len(toolNames) == 0 {
		return errors.New("Tool allowlist cannot be empty")
	}
	seen := make(map[string]struct{}, len(toolNames))
	for _, toolName := range toolNames {
		if strings.TrimSpace(toolName) == "" {
			return errors.New("Tool allowlist cannot contain an empty tool name")
		}
		if _, exists := seen[toolName]; exists {
			return fmt.Errorf("Tool allowlist contains duplicate tool %q", toolName)
		}
		seen[toolName] = struct{}{}
	}
	return nil
}
