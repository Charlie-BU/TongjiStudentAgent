// Package taskplan 定义会话级任务计划的领域模型、校验与可信访问范围。
package taskplan

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrTaskPlanNotFound 表示当前会话没有活动任务计划。
	ErrTaskPlanNotFound = errors.New("session task plan not found")
	// ErrTaskPlanConflict 表示计划版本已被更新，调用方应重新读取后再提交。
	ErrTaskPlanConflict = errors.New("session task plan revision conflict")
	// ErrInvalidTaskPlan 表示任务计划或任务项不符合持久化约束。
	ErrInvalidTaskPlan = errors.New("session task plan is invalid")
)

const (
	maxTaskPlanTasks       = 20  // 任务计划最大任务数
	maxTaskPlanDescription = 160 // 任务计划最大任务描述长度
)

var taskPlanIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// TaskStatus 表示任务计划中单项任务的执行状态。
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

// TaskItem 是任务计划中一个面向用户展示的步骤。
type TaskItem struct {
	ID     string     `json:"id"`
	Desc   string     `json:"desc"`
	Status TaskStatus `json:"status"`
}

// TaskPlan 是独立于 canonical 消息历史的会话编排状态。
// Revision 用于阻止脱离当前快照的覆盖写入。
type TaskPlan struct {
	SessionID string     `json:"session_id"`
	Revision  int64      `json:"revision"`
	Tasks     []TaskItem `json:"tasks"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ValidateTaskPlanTasks 规范化并校验完整任务列表。
func ValidateTaskPlanTasks(tasks []TaskItem) ([]TaskItem, error) {
	if len(tasks) == 0 || len(tasks) > maxTaskPlanTasks {
		return nil, ErrInvalidTaskPlan
	}
	result := make([]TaskItem, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		task.ID = strings.TrimSpace(task.ID)
		task.Desc = strings.TrimSpace(task.Desc)
		if !taskPlanIDPattern.MatchString(task.ID) || task.Desc == "" || utf8.RuneCountInString(task.Desc) > maxTaskPlanDescription || !isTaskStatus(task.Status) {
			return nil, fmt.Errorf("%w: invalid task item", ErrInvalidTaskPlan)
		}
		if _, exists := seen[task.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate task ID %q", ErrInvalidTaskPlan, task.ID)
		}
		seen[task.ID] = struct{}{}
		result = append(result, task)
	}
	return result, nil
}

// isTaskStatus 校验任务状态是否有效。
func isTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusDone, TaskStatusFailed:
		return true
	default:
		return false
	}
}
