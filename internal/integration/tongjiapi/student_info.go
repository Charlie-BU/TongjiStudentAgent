package tongjiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// studentInfoPath 是 Agent 在模型运行前读取当前授权学生资料的边界豁免接口。
const studentInfoPath = "/v1/rt/user/all_student"

// StudentInfo 表示 Agent 上下文所需的当前授权学生基础资料。
// 未列出的上游字段不得向上层传递或注入模型输入。
type StudentInfo struct {
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

// GetStudentInfo 调用边界豁免的当前授权学生资料接口。
func (c *Client) GetStudentInfo(ctx context.Context, accessToken string) (*StudentInfo, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("access token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.APIBaseURL+studentInfoPath, bytes.NewBufferString("{}"))
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
	var students []StudentInfo
	if err := json.Unmarshal(response.Data, &students); err != nil {
		return nil, fmt.Errorf("unmarshal student info response: %w", err)
	}
	if len(students) == 0 {
		return nil, fmt.Errorf("student info response is empty")
	}
	return &students[0], nil
}

// FormatStudentInfo 裁剪为可直接注入可信上下文的稳定学生资料文本。
func FormatStudentInfo(studentInfo *StudentInfo) string {
	if studentInfo == nil {
		return ""
	}
	parts := make([]string, 0, 27)
	appendField := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, label+"："+value)
		}
	}
	appendField("生日", studentInfo.Birthday)
	appendField("是否为港澳台侨", studentInfo.ChinaSon)
	appendField("培养专业", studentInfo.CultureProfession)
	if studentInfo.CurrentGrade > 0 {
		parts = append(parts, fmt.Sprintf("当前年级：%d", studentInfo.CurrentGrade))
	}
	appendField("入学时间", studentInfo.EnrolDate)
	appendField("入学方式", studentInfo.EnrolMethods)
	appendField("入学季节", studentInfo.EnrolSeason)
	appendField("预计毕业时间", studentInfo.ExpectedGraduationDate)
	appendField("学院", studentInfo.Faculty)
	appendField("学习形式", studentInfo.FormLearning)
	appendField("户籍注册地", studentInfo.HouseholdRegister)
	appendField("是否双学位", studentInfo.IsDobleDegree)
	appendField("是否是国际生", studentInfo.IsOverseas)
	appendField("在校状态", studentInfo.LeaveSchool)
	appendField("学制（年）", studentInfo.LengthSchooling)
	appendField("家庭地址", studentInfo.MailingAddress)
	appendField("专业方向", studentInfo.MajorDirection)
	appendField("姓名", studentInfo.Name)
	appendField("姓名拼音", studentInfo.NameSpelling)
	appendField("民族", studentInfo.Nation)
	appendField("政治面貌", studentInfo.PoliticalStatus)
	appendField("性别", studentInfo.Sex)
	appendField("专项计划", studentInfo.SpcialPlan)
	appendField("国籍", studentInfo.State)
	appendField("学（工）号", studentInfo.StudentID)
	appendField("培养类别", studentInfo.TrainingCategory)
	appendField("培养层次", studentInfo.TrainingLevel)
	return strings.Join(parts, "\n")
}
