package chat

import (
	"context"
	"fmt"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
	platformauth "github.com/Charlie-BU/TongjiStudent/internal/platform/auth"
)

// loadFormattedStudentInfo 仅使用本轮可信用户 ID 和服务凭据查询学生资料。
func (s *Service) loadFormattedStudentInfo(ctx context.Context) (string, error) {
	userID, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return "", nil
	}
	if s.tongjiClient == nil {
		return "", fmt.Errorf("Tongji Open Platform client is not initialized")
	}
	accessToken, err := tongjiapi.MCPAccessToken(ctx)
	if err != nil {
		return "", err
	}
	studentInfo, err := s.tongjiClient.GetStudentInfo(ctx, accessToken, userID)
	if err != nil {
		return "", fmt.Errorf("get Tongji student info: %w", err)
	}
	return tongjiapi.FormatStudentInfo(studentInfo), nil
}
