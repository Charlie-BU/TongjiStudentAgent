// Package auth 提供请求级校园授权凭据上下文。
package auth

import (
	"context"
	"fmt"
	"strings"
)

type accessTokenContextKey struct{}

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

// WithAccessToken 将已规范化的校园访问凭据写入请求上下文。
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, accessTokenContextKey{}, accessToken)
}

// AccessTokenFromContext 读取请求上下文中的校园访问凭据。
func AccessTokenFromContext(ctx context.Context) (string, bool) {
	accessToken, ok := ctx.Value(accessTokenContextKey{}).(string)
	return accessToken, ok && accessToken != ""
}
