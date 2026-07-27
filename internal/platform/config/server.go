// Package config 管理服务运行配置。
package config

import (
	"os"
	"strconv"
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
