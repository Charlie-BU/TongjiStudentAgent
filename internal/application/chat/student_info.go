package chat

import (
	"context"
	"fmt"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
)

// loadFormattedStudentInfo 仅在请求上下文携带 access token 时读取学生基础信息。
func (s *Service) loadFormattedStudentInfo(ctx context.Context) (string, error) {
	accessToken, ok := platformauth.AccessTokenFromContext(ctx)
	if !ok {
		return "", nil
	}
	if s.tongjiClient == nil {
		return "", fmt.Errorf("Tongji Open Platform client is not initialized")
	}
	studentInfo, err := s.tongjiClient.GetStudentInfo(ctx, accessToken)
	if err != nil {
		return "", fmt.Errorf("get Tongji student info: %w", err)
	}
	return tongjiapi.FormatStudentInfo(studentInfo), nil
}
