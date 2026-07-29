// Package prompt 集中维护允许调用的 prompt 白名单。
package prompt

const (
	TongjiStudentSystemPrompt = "prompt.tongjistudent.system_prompt"
)

var (
	Prompts = []string{TongjiStudentSystemPrompt}
)
