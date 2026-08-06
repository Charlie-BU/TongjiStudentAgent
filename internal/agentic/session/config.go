package session

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	anonymousSessionTTLEnv          = "SESSION_ANONYMOUS_TTL"
	anonymousSessionMessageLimitEnv = "SESSION_ANONYMOUS_MAX_MESSAGES"
	historyMessageLimitEnv          = "SESSION_HISTORY_MAX_MESSAGES"
	defaultAnonymousSessionTTL      = 24 * time.Hour
	defaultAnonymousMessageLimit    = 20
	defaultHistoryMessageLimit      = 20
)

// Config 描述会话存储的容量与历史窗口配置。
type Config struct {
	AnonymousTTL          time.Duration
	AnonymousMessageLimit int
	HistoryMessageLimit   int
}

// ConfigFromEnv 从环境变量读取会话容量与历史窗口配置。
func ConfigFromEnv() (Config, error) {
	config := Config{
		AnonymousTTL:          defaultAnonymousSessionTTL,
		AnonymousMessageLimit: defaultAnonymousMessageLimit,
		HistoryMessageLimit:   defaultHistoryMessageLimit,
	}
	if value := strings.TrimSpace(os.Getenv(anonymousSessionTTLEnv)); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration", anonymousSessionTTLEnv)
		}
		config.AnonymousTTL = parsed
	}
	for _, item := range []struct {
		key         string
		destination *int
	}{
		{key: anonymousSessionMessageLimitEnv, destination: &config.AnonymousMessageLimit},
		{key: historyMessageLimitEnv, destination: &config.HistoryMessageLimit},
	} {
		if value := strings.TrimSpace(os.Getenv(item.key)); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive integer", item.key)
			}
			*item.destination = parsed
		}
	}
	return config, nil
}
