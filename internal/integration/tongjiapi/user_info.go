package tongjiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// userInfoPath 是 Agent 在模型运行前读取当前授权学生资料的边界豁免接口。
const userInfoPath = "/v1/rt/user/all_student"

// UserInfo 表示 Agent 上下文所需的当前授权学生基础资料。
// 未列出的上游字段不得向上层传递或注入模型输入。
type UserInfo struct {
	Birthday               string `json:"birthday"`
	ChinaSon               string `json:"chinaSon"`
	CultureProfession      string `json:"cultureProfession"`
	CurrentGrade           int    `json:"currentGrade"`
	EnrolDate              string `json:"enrolDate"`
	EnrolMethods           string `json:"enrolMethods"`
	EnrolSeason            string `json:"enrolSeason"`
	ExpectedGraduationDate string `json:"expectedGraduationDate"`
	Faculty                string `json:"faculty"`
	FormLearning           string `json:"formLearning"`
	HouseholdRegister      string `json:"householdRegister"`
	IsDobleDegree          string `json:"isDobleDegree"`
	IsOverseas             string `json:"isOverseas"`
	LeaveSchool            string `json:"leaveSchool"`
	LengthSchooling        string `json:"lengthSchooling"`
	MailingAddress         string `json:"mailingAddress"`
	MajorDirection         string `json:"majorDirection"`
	Name                   string `json:"name"`
	NameSpelling           string `json:"nameSpelling"`
	Nation                 string `json:"nation"`
	PoliticalStatus        string `json:"politicalStatus"`
	Sex                    string `json:"sex"`
	SpcialPlan             string `json:"spcialPlan"`
	State                  string `json:"state"`
	StudentID              string `json:"studentId"`
	TrainingCategory       string `json:"trainingCategory"`
	TrainingLevel          string `json:"trainingLevel"`
}

// GetUserInfo 调用边界豁免的当前授权学生资料接口。
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("access token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.APIBaseURL+userInfoPath, bytes.NewBufferString("{}"))
	if err != nil {
		return nil, fmt.Errorf("create user info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := c.doAPIRequest(req)
	if err != nil {
		return nil, err
	}
	var users []UserInfo
	if err := json.Unmarshal(response.Data, &users); err != nil {
		return nil, fmt.Errorf("unmarshal user info response: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user info response is empty")
	}
	return &users[0], nil
}

// FormatUserInfo 裁剪为可直接注入可信上下文的稳定用户资料文本。
func FormatUserInfo(userInfo *UserInfo) string {
	if userInfo == nil {
		return ""
	}
	parts := make([]string, 0, 27)
	appendField := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, label+"："+value)
		}
	}
	appendField("生日", userInfo.Birthday)
	appendField("是否为港澳台侨", userInfo.ChinaSon)
	appendField("培养专业", userInfo.CultureProfession)
	if userInfo.CurrentGrade > 0 {
		parts = append(parts, fmt.Sprintf("当前年级：%d", userInfo.CurrentGrade))
	}
	appendField("入学时间", userInfo.EnrolDate)
	appendField("入学方式", userInfo.EnrolMethods)
	appendField("入学季节", userInfo.EnrolSeason)
	appendField("预计毕业时间", userInfo.ExpectedGraduationDate)
	appendField("学院", userInfo.Faculty)
	appendField("学习形式", userInfo.FormLearning)
	appendField("户籍注册地", userInfo.HouseholdRegister)
	appendField("是否双学位", userInfo.IsDobleDegree)
	appendField("是否是国际生", userInfo.IsOverseas)
	appendField("在校状态", userInfo.LeaveSchool)
	appendField("学制（年）", userInfo.LengthSchooling)
	appendField("家庭地址", userInfo.MailingAddress)
	appendField("专业方向", userInfo.MajorDirection)
	appendField("姓名", userInfo.Name)
	appendField("姓名拼音", userInfo.NameSpelling)
	appendField("民族", userInfo.Nation)
	appendField("政治面貌", userInfo.PoliticalStatus)
	appendField("性别", userInfo.Sex)
	appendField("专项计划", userInfo.SpcialPlan)
	appendField("国籍", userInfo.State)
	appendField("学（工）号", userInfo.StudentID)
	appendField("培养类别", userInfo.TrainingCategory)
	appendField("培养层次", userInfo.TrainingLevel)
	return strings.Join(parts, "\n")
}
