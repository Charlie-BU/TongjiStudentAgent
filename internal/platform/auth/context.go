// Package auth 提供请求级校园授权凭据上下文。
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/Charlie-BU/TongjiStudent/internal/integration/tongjiapi"
)

type accessTokenContextKey struct{}
type userIDContextKey struct{}

// userIDResolver 通过当前请求的访问凭据解析可信用户 ID。
type userIDResolver func(ctx context.Context, accessToken string) (string, error)

var resolveUserID userIDResolver = resolveUserIDFromTongji

// ExtractBearerToken 解析 HTTP Authorization 头中的 Bearer 凭据。
func ExtractBearerToken(authorization string) (string, error) {
	parts := strings.Fields(authorization)
	if len(parts) == 0 {
		return "", fmt.Errorf("authorization token is required")
	}
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", fmt.Errorf("authorization token must use Bearer scheme")
	}
	return parts[1], nil
}

// WithAccessToken 将已规范化的校园访问凭据写入请求上下文，并尽力补充可信用户 ID。
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, accessTokenContextKey{}, accessToken)
	userID, err := resolveUserID(ctx, accessToken)
	if err != nil || strings.TrimSpace(userID) == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

// AccessTokenFromContext 读取请求上下文中的校园访问凭据。
func AccessTokenFromContext(ctx context.Context) (string, bool) {
	accessToken, ok := ctx.Value(accessTokenContextKey{}).(string)
	return accessToken, ok && accessToken != ""
}

// UserIDFromContext 读取由用户基础信息接口解析出的可信用户 ID。
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok && userID != ""
}

// resolveUserIDFromTongji 调用用户基础信息豁免接口并返回当前授权用户 ID。
func resolveUserIDFromTongji(ctx context.Context, accessToken string) (string, error) {
	client, err := tongjiapi.NewFromEnv()
	if err != nil {
		return "", fmt.Errorf("create Tongji Open Platform client: %w", err)
	}
	userBasicInfo, err := client.GetUserBasicInfo(ctx, accessToken)
	if err != nil {
		return "", fmt.Errorf("get Tongji user basic info: %w", err)
	}
	userID := strings.TrimSpace(userBasicInfo.UserId)
	if userID == "" {
		return "", fmt.Errorf("Tongji user basic info did not include user ID")
	}
	return userID, nil
}
