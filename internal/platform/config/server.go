// Package config 管理服务运行配置。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// GracefulTime 返回服务优雅退出等待时长。
func GracefulTime() time.Duration {
	gracefulTimeout, err := strconv.Atoi(os.Getenv("FUNC_TIMEOUT"))
	if err != nil {
		gracefulTimeout = 30
	}
	return time.Duration(gracefulTimeout) * time.Second
}

// ServerPort 返回 HTTP 服务监听端口。
func ServerPort() string {
	if port := os.Getenv("PORT0"); port != "" {
		return port
	}
	return "8080"
}

// CORSAllowOrigins 返回允许跨域访问服务的 Origin 白名单。
//
// 环境变量 CORS_ALLOW_ORIGINS 使用 JSON 字符串数组，例如：
// '["https://app.tongji.edu.cn", "http://localhost:5173"]'。未配置时不启用 CORS。
func CORSAllowOrigins() ([]string, error) {
	rawOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
	if rawOrigins == "" {
		return nil, nil
	}

	var origins []string
	if err := json.Unmarshal([]byte(rawOrigins), &origins); err != nil {
		return nil, fmt.Errorf("CORS_ALLOW_ORIGINS must be a JSON string array: %w", err)
	}
	for index, origin := range origins {
		origins[index] = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origins[index] == "" {
			return nil, fmt.Errorf("CORS_ALLOW_ORIGINS contains an empty origin")
		}
	}
	return origins, nil
}
