// Package managetaskplan 提供会话级任务计划管理静态系统工具。
package managetaskplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	agentevent "github.com/Charlie-BU/TongjiStudent/internal/agentic/event"
	taskplan "github.com/Charlie-BU/TongjiStudent/internal/agentic/session/taskplan"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const ManageTaskPlanToolName = toolallowlist.ManageTaskPlanTool

type manageTaskPlanTool struct {
	isToolAllowed func(string) bool
	repository    taskplan.TaskPlanRepository
}

type arguments struct {
	Action string              `json:"action"`
	Tasks  []taskplan.TaskItem `json:"tasks,omitempty"`
	Reason string              `json:"reason"`
}

type result struct {
	Status   string              `json:"status"`
	Action   string              `json:"action,omitempty"`
	Revision int64               `json:"revision,omitempty"`
	Tasks    []taskplan.TaskItem `json:"tasks,omitempty"`
	Message  string              `json:"message,omitempty"`
}

// NewTool 创建受应用 allowlist 与会话 scope 共同保护的任务计划工具。
func NewTool(isToolAllowed func(string) bool, repository taskplan.TaskPlanRepository) *manageTaskPlanTool {
	return &manageTaskPlanTool{isToolAllowed: isToolAllowed, repository: repository}
}

// Info 实现 tool.BaseTool，向模型声明完整任务计划操作协议。
func (*manageTaskPlanTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ManageTaskPlanToolName,
		Desc: "管理当前已授权会话的任务执行计划。仅在任务包含多个步骤、需要进度追踪或用户切换目标时调用。reason 和任务描述会被用户看到，只能写简短操作说明，禁止包含私有推理、凭据或敏感信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Desc:     "操作类型：create（无活动计划时创建完整计划）、modify（以完整计划替换当前计划）、update_status（仅更新已有任务状态）、append（追加任务）、clear（清空当前计划）。",
				Required: true,
				Type:     schema.String,
				Enum:     []string{"create", "modify", "update_status", "append", "clear"},
			},
			"tasks": {
				Desc: "create、modify、append 时提供完整任务项；update_status 时只提供已有任务的 id 和 status。任务项包含 id、desc、status（pending、in_progress、done、failed）。",
				Type: schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.Object, SubParams: map[string]*schema.ParameterInfo{
					"id":     {Desc: "任务唯一 ID，例如 step1。", Type: schema.String},
					"desc":   {Desc: "面向用户的简短任务描述。update_status 时不得提供。", Type: schema.String},
					"status": {Desc: "任务状态。", Type: schema.String, Enum: []string{"pending", "in_progress", "done", "failed"}},
				}},
			},
			"reason": {
				Desc:     "面向用户的简短操作说明；不得包含私有推理、凭据或敏感信息。",
				Required: true,
				Type:     schema.String,
			},
		}),
	}, nil
}

// InvokableRun 执行当前会话范围内的任务计划状态变更。
func (t *manageTaskPlanTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.isToolAllowed == nil || !t.isToolAllowed(ManageTaskPlanToolName) {
		return encodeResult(result{Status: "tool_not_allowed", Message: "当前环境未启用该系统工具。"})
	}
	if t.repository == nil {
		return "", errors.New("task plan repository is not initialized")
	}
	input, err := decodeArguments(argumentsInJSON)
	if err != nil {
		return encodeResult(result{Status: "invalid_arguments", Message: "任务计划参数无效。"})
	}
	if strings.TrimSpace(input.Reason) == "" {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "reason 为必填的简短操作说明。"})
	}

	switch input.Action {
	case "create":
		return t.create(ctx, input)
	case "modify":
		return t.modify(ctx, input)
	case "update_status":
		return t.updateStatus(ctx, input)
	case "append":
		return t.append(ctx, input)
	case "clear":
		return t.clear(ctx, input)
	default:
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "action 无效。"})
	}
}

// create 创建新任务计划。
func (t *manageTaskPlanTool) create(ctx context.Context, input arguments) (string, error) {
	if len(input.Tasks) == 0 {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "create 必须提供 tasks。"})
	}
	current, err := t.repository.GetTaskPlan(ctx)
	if err != nil {
		return "", fmt.Errorf("get task plan before create: %w", err)
	}
	if current != nil {
		return encodeResult(result{Status: "active_plan_exists", Action: input.Action, Revision: current.Revision, Tasks: current.Tasks, Message: "当前已有活动计划，请使用 modify、append 或 update_status。"})
	}
	return t.save(ctx, input.Action, 0, input.Tasks)
}

// modify 以完整计划替换当前计划。
func (t *manageTaskPlanTool) modify(ctx context.Context, input arguments) (string, error) {
	if len(input.Tasks) == 0 {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "modify 必须提供完整 tasks。"})
	}
	current, err := t.repository.GetTaskPlan(ctx)
	if err != nil {
		return "", fmt.Errorf("get task plan before modify: %w", err)
	}
	if current == nil {
		return encodeResult(result{Status: "no_active_plan", Action: input.Action, Message: "当前没有活动计划，请使用 create。"})
	}
	return t.save(ctx, input.Action, current.Revision, input.Tasks)
}

// updateStatus 更新已有任务状态。
func (t *manageTaskPlanTool) updateStatus(ctx context.Context, input arguments) (string, error) {
	if len(input.Tasks) == 0 {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "update_status 必须提供 tasks。"})
	}
	current, err := t.repository.GetTaskPlan(ctx)
	if err != nil {
		return "", fmt.Errorf("get task plan before update status: %w", err)
	}
	if current == nil {
		return encodeResult(result{Status: "no_active_plan", Action: input.Action, Message: "当前没有活动计划。"})
	}
	updates := make(map[string]taskplan.TaskStatus, len(input.Tasks))
	for _, task := range input.Tasks {
		if strings.TrimSpace(task.ID) == "" || task.Status == "" || strings.TrimSpace(task.Desc) != "" {
			return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "update_status 仅接受已有任务的 id 和 status。"})
		}
		if _, exists := updates[task.ID]; exists {
			return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "update_status 不得包含重复任务 ID。"})
		}
		updates[task.ID] = task.Status
	}
	updatedTasks := append([]taskplan.TaskItem(nil), current.Tasks...)
	for index := range updatedTasks {
		if status, exists := updates[updatedTasks[index].ID]; exists {
			updatedTasks[index].Status = status
			delete(updates, updatedTasks[index].ID)
		}
	}
	if len(updates) > 0 {
		return encodeResult(result{Status: "unknown_task_id", Action: input.Action, Message: "update_status 只能更新已有任务。"})
	}
	return t.save(ctx, input.Action, current.Revision, updatedTasks)
}

// append 向当前计划追加任务。
func (t *manageTaskPlanTool) append(ctx context.Context, input arguments) (string, error) {
	if len(input.Tasks) == 0 {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "append 必须提供 tasks。"})
	}
	current, err := t.repository.GetTaskPlan(ctx)
	if err != nil {
		return "", fmt.Errorf("get task plan before append: %w", err)
	}
	if current == nil {
		return encodeResult(result{Status: "no_active_plan", Action: input.Action, Message: "当前没有活动计划，请使用 create。"})
	}
	return t.save(ctx, input.Action, current.Revision, append(current.Tasks, input.Tasks...))
}

// clear 清空当前任务计划。
func (t *manageTaskPlanTool) clear(ctx context.Context, input arguments) (string, error) {
	if len(input.Tasks) != 0 {
		return encodeResult(result{Status: "invalid_arguments", Action: input.Action, Message: "clear 不接受 tasks。"})
	}
	current, err := t.repository.GetTaskPlan(ctx)
	if err != nil {
		return "", fmt.Errorf("get task plan before clear: %w", err)
	}
	if current == nil {
		return encodeResult(result{Status: "no_active_plan", Action: input.Action, Message: "当前没有活动计划。"})
	}
	if err := t.repository.ClearTaskPlan(ctx, current.Revision); err != nil {
		return "", fmt.Errorf("clear task plan: %w", err)
	}
	agentevent.EmitFromContext(ctx, agentevent.TaskPlanUpdated, agentevent.TaskPlanUpdatedData{Action: input.Action, Revision: current.Revision, Tasks: []taskplan.TaskItem{}})
	return encodeResult(result{Status: "cleared", Action: input.Action, Message: "任务计划已清空。"})
}

// save 保存当前任务计划。
func (t *manageTaskPlanTool) save(ctx context.Context, action string, revision int64, tasks []taskplan.TaskItem) (string, error) {
	plan, err := t.repository.SaveTaskPlan(ctx, revision, tasks)
	if err != nil {
		if errors.Is(err, taskplan.ErrInvalidTaskPlan) {
			return encodeResult(result{Status: "invalid_arguments", Action: action, Message: "任务列表不符合约束。"})
		}
		if errors.Is(err, taskplan.ErrTaskPlanConflict) {
			return encodeResult(result{Status: "plan_conflict", Action: action, Message: "任务计划已更新，请先重新读取当前计划。"})
		}
		return "", fmt.Errorf("save task plan: %w", err)
	}
	agentevent.EmitFromContext(ctx, agentevent.TaskPlanUpdated, agentevent.TaskPlanUpdatedData{Action: action, Revision: plan.Revision, Tasks: plan.Tasks})
	return encodeResult(result{Status: "updated", Action: action, Revision: plan.Revision, Tasks: plan.Tasks, Message: "任务计划已更新。"})
}

// decodeArguments 解析 JSON 字符串为 arguments 结构体。
func decodeArguments(argumentsInJSON string) (arguments, error) {
	decoder := json.NewDecoder(strings.NewReader(argumentsInJSON))
	decoder.DisallowUnknownFields()
	var input arguments
	if err := decoder.Decode(&input); err != nil {
		return arguments{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return arguments{}, errors.New("multiple JSON values")
	}
	return input, nil
}

// encodeResult 序列化 result 结构体为 JSON 字符串。
func encodeResult(value result) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode task plan result: %w", err)
	}
	return string(encoded), nil
}
