package systemtools

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

type loadSkillTool struct {
	isToolAllowed func(string) bool
}

type loadSkillArguments struct {
	SkillID string `json:"skill_id"`
	Reason  string `json:"reason"`
}

// newLoadSkillTool 创建由应用 Tool allowlist 保护的 Skill 加载工具。
func newLoadSkillTool(isToolAllowed func(string) bool) *loadSkillTool {
	return &loadSkillTool{isToolAllowed: isToolAllowed}
}

// Info 返回 system.load_skill 的定义。
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

// InvokableRun 读取 allowlist 中 Skill 的固定主手册，不接受任意文件路径。
func (t *loadSkillTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t == nil || t.isToolAllowed == nil || !t.isToolAllowed(LoadSkillToolName) {
		return loadSkillResult("tool_not_allowed", "当前环境未启用该系统工具。"), nil
	}

	var arguments loadSkillArguments
	decoder := json.NewDecoder(strings.NewReader(argumentsInJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&arguments); err != nil {
		return loadSkillResult("invalid_arguments", "Skill 加载参数无效。"), nil
	}
	skillID := strings.TrimSpace(arguments.SkillID)
	if skillID == "" || strings.TrimSpace(arguments.Reason) == "" {
		return loadSkillResult("invalid_arguments", "skill_id 和 reason 均为必填项。"), nil
	}
	if !skillallowlist.IsAllowedSkill(skillID) {
		return loadSkillResult("skill_not_allowed", "请求的 Skill 未获授权。"), nil
	}

	content, err := agenticskills.Load(skillID)
	if err != nil {
		return loadSkillResult("skill_unavailable", "Skill 当前不可用，请稍后重试。"), nil
	}
	result, err := json.Marshal(struct {
		Status  string `json:"status"`
		SkillID string `json:"skill_id"`
		Content string `json:"content"`
	}{Status: "ok", SkillID: skillID, Content: content})
	if err != nil {
		return loadSkillResult("skill_unavailable", "Skill 当前不可用，请稍后重试。"), nil
	}
	return string(result), nil
}

func loadSkillResult(status string, message string) string {
	result, err := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{Status: status, Message: message})
	if err != nil {
		return `{"status":"skill_unavailable","message":"Skill 当前不可用，请稍后重试。"}`
	}
	return string(result)
}
