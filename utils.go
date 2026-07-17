package main

import (
	"os"
	"strconv"
	"time"
)

func ptrOf[T any](v T) *T {
	return &v
}

func GetGracefulTime() time.Duration {
	gracefulTimeout, err := strconv.Atoi(os.Getenv("_BYTEFAAS_FUNC_TIMEOUT"))
	if err != nil {
		gracefulTimeout = 30
	}
	return time.Duration(gracefulTimeout) * time.Second
}

func GetServerPort() string {
	if port := os.Getenv("PORT0"); port != "" {
		return port
	}
	return "8080"
}
