package loadskill

import (
	"context"
	"encoding/json"
	"strings"

	agenticskills "github.com/Charlie-BU/TongjiStudent/internal/agentic/skills"
	skillallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/skill"
	toolallowlist "github.com/Charlie-BU/TongjiStudent/internal/application/allowlist/tool"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const LoadSkillToolName = toolallowlist.LoadSkillTool

// loadSkillTool 通过实现 tool.BaseTool 和 tool.InvokableTool 的方法集，被 ToolsNode 识别为一个可声明且可执行的工具。
type loadSkillTool struct {
	isToolAllowed func(string) bool
}

type loadSkillArguments struct {
	SkillID string `json:"skill_id"`
	Reason  string `json:"reason"`
}

type loadSkillStatus string

const (
	loadSkillStatusOK               loadSkillStatus = "ok"
	loadSkillStatusAlreadyLoaded    loadSkillStatus = "already_loaded"
	loadSkillStatusSkillNotAllowed  loadSkillStatus = "skill_not_allowed"
	loadSkillStatusRunUnavailable   loadSkillStatus = "skill_run_unavailable"
	loadSkillStatusSkillUnavailable loadSkillStatus = "skill_unavailable"
	loadSkillStatusInvalidArguments loadSkillStatus = "invalid_arguments"
	loadSkillStatusToolNotAllowed   loadSkillStatus = "tool_not_allowed"
)

// NewTool 创建由应用 Tool allowlist 保护的 Skill 加载工具。
func NewTool(isToolAllowed func(string) bool) *loadSkillTool {
	return &loadSkillTool{isToolAllowed: isToolAllowed}
}

// Info 实现 tool.BaseTool，用于向模型暴露工具名、描述和参数定义。
func (t *loadSkillTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: LoadSkillToolName,
		Desc: "加载已批准的 Skill 手册，使其可在当前 Agent Run 中作为受信任工作说明使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill_id": {
				Desc:     "应用 allowlist 中已批准的 Skill ID。",
				Required: true,
				Type:     schema.String,
			},
			"reason": {
				Desc:     "加载此 Skill 的简短原因。",
				Required: true,
				Type:     schema.String,
			},
		}),
	}, nil
}

// InvokableRun 实现 tool.InvokableTool，负责执行工具调用并返回字符串结果。
// 它只读取 allowlist 中 Skill 的固定主手册，不接受任意文件路径。
func (t *loadSkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	// 检查当前 load_skill 工具是否启用
	if t == nil || t.isToolAllowed == nil || !t.isToolAllowed(LoadSkillToolName) {
		return loadSkillResult(loadSkillStatusToolNotAllowed, "当前环境未启用该系统工具。"), nil
	}

	var arguments loadSkillArguments
	decoder := json.NewDecoder(strings.NewReader(argumentsInJSON))
	decoder.DisallowUnknownFields() // 只允许 skill_id 和 reason 两个字段
	if err := decoder.Decode(&arguments); err != nil {
		return loadSkillResult(loadSkillStatusInvalidArguments, "Skill 加载参数无效。"), nil
	}
	skillID := strings.TrimSpace(arguments.SkillID)

	if skillID == "" || strings.TrimSpace(arguments.Reason) == "" {
		return loadSkillResult(loadSkillStatusInvalidArguments, "skill_id 和 reason 均为必填项。"), nil
	}
	// 检查本 Skill 是否被开白
	if !skillallowlist.IsAllowedSkill(skillID) {
		return loadSkillResult(loadSkillStatusSkillNotAllowed, "请求的 Skill 未获授权。"), nil
	}

	// 获取当前 Run 的 Skill 状态，检查本 Skill 是否已加载
	runState, ok := agenticskills.RunStateFromContext(ctx)
	if !ok {
		return loadSkillResult(loadSkillStatusRunUnavailable, "当前 Skill Run 状态不可用。"), nil
	}
	content, alreadyLoaded, err := runState.LoadOnce(skillID, func() (string, error) {
		return agenticskills.Load(skillID)
	})
	if err != nil {
		return loadSkillResult(loadSkillStatusSkillUnavailable, "Skill 当前不可用，请稍后重试。"), nil
	}
	if alreadyLoaded {
		return loadSkillAlreadyLoadedResult(skillID), nil
	}
	result, err := json.Marshal(struct {
		Status  loadSkillStatus `json:"status"`
		SkillID string          `json:"skill_id"`
		Content string          `json:"content"`
	}{Status: loadSkillStatusOK, SkillID: skillID, Content: content})
	if err != nil {
		return loadSkillResult(loadSkillStatusSkillUnavailable, "Skill 当前不可用，请稍后重试。"), nil
	}
	return string(result), nil
}

// loadSkillAlreadyLoadedResult 返回 load_skill 工具的执行结果，表示本 Skill 已在当前 Run 加载，无需重复加载。
func loadSkillAlreadyLoadedResult(skillID string) string {
	result, err := json.Marshal(struct {
		Status  loadSkillStatus `json:"status"`
		SkillID string          `json:"skill_id"`
		Message string          `json:"message"`
	}{
		Status:  loadSkillStatusAlreadyLoaded,
		SkillID: skillID,
		Message: "Skill 已在当前 Run 加载，无需重复加载。",
	})
	if err != nil {
		return `{"status":"already_loaded","message":"Skill 已在当前 Run 加载，无需重复加载。"}`
	}
	return string(result)
}

// loadSkillResult 返回 load_skill 工具的执行结果。
func loadSkillResult(status loadSkillStatus, message string) string {
	result, err := json.Marshal(struct {
		Status  loadSkillStatus `json:"status"`
		Message string          `json:"message"`
	}{Status: status, Message: message})
	if err != nil {
		return `{"status":"skill_unavailable","message":"Skill 当前不可用，请稍后重试。"}`
	}
	return string(result)
}
