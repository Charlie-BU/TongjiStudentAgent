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
	// 远程 MCP Tool
	TongjiAnnualBillTool          = "tongji.student.annual_bill"
	TongjiCardSpendingFlowTool    = "tongji.student.card_spending_flow"
	TongjiStudentTimetableTool    = "tongji.student.timetable"
	TongjiStudentDetailedInfoTool = "tongji.student.detailed_info"
	TongjiStudentScoreTool        = "tongji.student.score"
	TongjiTermCalendarTool        = "tongji.student.term-calendar"
	TongjiCurrentTermCalendarTool = "tongji.student.current-term-calendar"
	TongjiCETScoreTool            = "tongji.student.cet-score"
	TongjiBookLendInfoTool        = "tongji.student.book-lend-info"
	TongjiStatisticsInfoTool      = "tongji.student.statistics-info"
	TongjiStipendInfoTool         = "tongji.student.stipend-info"
	TongjiAccommodationInfoTool   = "tongji.student.accommodation-info"
	TongjiCompetitionPrizeTool    = "tongji.student.competition_prize"
	TongjiHonoraryTitleTool       = "tongji.student.honorary_title"
	TongjiScholarshipInfoTool     = "tongji.student.scholarship_info"
	TongjiSchoolAccessTool        = "tongji.student.school_access"
	TongjiLibraryAccessTool       = "tongji.student.library_access"
	TongjiUserBasicInfoTool       = "tongji.user.basic_info"
	TongjiCourseDetailTool        = "tongji.student.course-detail"
	TongjiCourseRelatedTool       = "tongji.student.course-related"
	TongjiFindMajorByGradeTool    = "tongji.student.find-major-by-grade"
	TongjiCourseCatalogTool       = "tongji.course.catalog"
	TongjiCalendarListTool        = "tongji.course.calendar_list"
	TongjiGradeListTool           = "tongji.course.grade_list"
)

var (
	allowedSystemTools = []string{
		LoadSkillTool,
		ManageTaskPlanTool,
		SearchKnowledgeTool,
	}

	allowedMCPTools = []string{
		TongjiAnnualBillTool,
		TongjiCardSpendingFlowTool,
		TongjiStudentTimetableTool,
		TongjiStudentDetailedInfoTool,
		TongjiStudentScoreTool,
		TongjiTermCalendarTool,
		TongjiCurrentTermCalendarTool,
		TongjiCETScoreTool,
		TongjiBookLendInfoTool,
		TongjiStatisticsInfoTool,
		TongjiStipendInfoTool,
		TongjiAccommodationInfoTool,
		TongjiCompetitionPrizeTool,
		TongjiHonoraryTitleTool,
		TongjiScholarshipInfoTool,
		TongjiSchoolAccessTool,
		TongjiLibraryAccessTool,
		TongjiUserBasicInfoTool,
		TongjiCourseDetailTool,
		TongjiCourseRelatedTool,
		TongjiFindMajorByGradeTool,
		TongjiCourseCatalogTool,
		TongjiCalendarListTool,
		TongjiGradeListTool,
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
