package skills

import (
	"context"
	"fmt"
	"sync"
)

type runStateContextKey struct{}

// RunState 记录单个 Agent Run 内已成功加载的 Skill。
// 它不应跨 Run 或会话复用。
type RunState struct {
	mu     sync.Mutex
	loaded map[string]struct{}
}

// NewRunState 创建本轮 Agent Run 独占的 Skill 状态。
func NewRunState() *RunState {
	return &RunState{loaded: make(map[string]struct{})}
}

// WithRunState 将本轮状态传递给静态系统工具。
func WithRunState(ctx context.Context, state *RunState) context.Context {
	return context.WithValue(ctx, runStateContextKey{}, state)
}

// RunStateFromContext 返回当前 Run 的 Skill 状态。
func RunStateFromContext(ctx context.Context) (*RunState, bool) {
	state, ok := ctx.Value(runStateContextKey{}).(*RunState)
	return state, ok && state != nil
}

// LoadOnce 加载一个尚未成功加载的 Skill。已加载的 Skill 不会再次执行 loader，且不会再次返回完整手册。
func (s *RunState) LoadOnce(skillID string, loader func() (string, error)) (content string, alreadyLoaded bool, err error) {
	if s == nil {
		return "", false, fmt.Errorf("skill run state is required")
	}
	if loader == nil {
		return "", false, fmt.Errorf("skill loader is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.loaded[skillID]; ok {
		return "", true, nil
	}

	content, err = loader()
	if err != nil {
		return "", false, err
	}
	s.loaded[skillID] = struct{}{}
	return content, false, nil
}
