// Package tavily 提供公开网页搜索与内容提取的 HTTP 适配器。
package tavily

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// NewFromEnv 根据开关和凭据创建客户端，关闭时返回 nil。
func NewFromEnv() (*Client, error) {
	enabled := false
	if value := strings.TrimSpace(os.Getenv("TAVILY_ENABLED")); value != "" {
		var err error
		enabled, err = strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("TAVILY_ENABLED must be a boolean")
		}
	}
	if !enabled {
		return nil, nil
	}
	return NewClient(os.Getenv("TAVILY_API_KEY"), 30*time.Second, nil)
}
