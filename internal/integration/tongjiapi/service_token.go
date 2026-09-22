package tongjiapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ServiceTokenCache 在进程内缓存服务端身份，并合并并发刷新。
type ServiceTokenCache struct {
	// mu 只保护以下缓存状态，不覆盖任何网络请求。
	mu        sync.Mutex
	token     string
	expiresAt time.Time
	// generation 每次刷新结束后递增，用来识别锁外校验所读取的缓存是否已被更新。
	generation uint64
	// refreshing 非 nil 表示已有请求负责刷新；关闭该 channel 会唤醒所有等待者。
	refreshing chan struct{}
}

var serviceTokens ServiceTokenCache

// MCPAccessToken 获取 MCP Access Token。
func MCPAccessToken(ctx context.Context) (string, error) {
	client := &Client{config: Config{
		ClientID:      strings.TrimSpace(os.Getenv("TONGJI_MCP_CLIENT_ID")),
		ClientSecret:  strings.TrimSpace(os.Getenv("TONGJI_MCP_CLIENT_SECRET")),
		TokenEndpoint: envOrDefault("TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT", defaultTokenEndpoint),
		APIBaseURL:    strings.TrimRight(envOrDefault("TONGJI_OPEN_PLATFORM_API_BASE_URL", defaultAPIBaseURL), "/"),
	}, httpClient: &http.Client{Timeout: defaultTimeout}}
	if client.config.ClientID == "" || client.config.ClientSecret == "" {
		return "", fmt.Errorf("TONGJI_MCP_CLIENT_ID and TONGJI_MCP_CLIENT_SECRET are required")
	}
	return serviceTokens.AccessToken(ctx, client)
}

// AccessToken 获取 MCP Access Token。
// 每次使用前校验服务身份；缓存缺失、过期或校验失败时刷新。
// 并发调用可以各自校验，但同一时刻只有一个调用负责刷新。
func (cache *ServiceTokenCache) AccessToken(ctx context.Context, client *Client) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		cache.mu.Lock()
		if done := cache.refreshing; done != nil {
			// 等待时释放锁，并允许本请求独立取消，不影响正在刷新的请求。
			cache.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-done:
				// 刷新可能成功，也可能失败或被取消；重新读取缓存决定下一步。
				continue
			}
		}
		// 在锁内取一致的快照，随后用该快照进行耗时的身份校验。
		token, expiresAt, generation := cache.token, cache.expiresAt, cache.generation
		cache.mu.Unlock()

		// 网络校验不持有缓存锁，让不同请求独立执行和取消。
		valid := token != "" && time.Now().Before(expiresAt)
		if valid {
			valid = checkServiceIdentity(ctx, client, token) == nil
		}
		// 请求取消不代表 Token 失效，不能因此清空其他请求正在使用的缓存。
		if err := ctx.Err(); err != nil {
			return "", err
		}
		cache.mu.Lock()
		// 版本变化表示其他请求已完成刷新；refreshing 非 nil 表示刷新仍在进行。
		// 两种情况都要丢弃本次校验结果，避免返回旧 Token 或重复刷新。
		if cache.generation != generation || cache.refreshing != nil {
			cache.mu.Unlock()
			continue
		}
		// 网络校验本身可能耗时，返回前再次确认快照中的 Token 尚未过期。
		if valid && time.Now().Before(expiresAt) {
			cache.mu.Unlock()
			return token, nil
		}
		// 在锁内登记本请求负责刷新并清空旧缓存，后续调用只能等待刷新结果。
		done := make(chan struct{})
		cache.refreshing = done
		cache.token, cache.expiresAt = "", time.Time{}
		cache.mu.Unlock()

		token, expiresAt, err := fetchServiceToken(ctx, client)
		cache.mu.Lock()
		// 成功时发布新 Token，失败时保留空缓存；两者都推进版本并唤醒等待者。
		// 必须先更新状态再通知，确保被唤醒的请求读取到本次刷新后的状态。
		cache.token, cache.expiresAt = token, expiresAt
		cache.generation++
		cache.refreshing = nil
		close(done)
		cache.mu.Unlock()
		return token, err
	}
}

// fetchServiceToken 申请并校验新 Token，不操作共享缓存；失败时返回空值。
func fetchServiceToken(ctx context.Context, client *Client) (string, time.Time, error) {
	token, err := client.requestToken(ctx, url.Values{
		"client_id": {client.config.ClientID}, "client_secret": {client.config.ClientSecret}, "grant_type": {"client_credentials"},
	})
	if err != nil {
		return "", time.Time{}, err
	}
	if err := checkServiceIdentity(ctx, client, token.AccessToken); err != nil {
		return "", time.Time{}, err
	}
	// 缺失或异常的有效期按两小时处理，同时将本地缓存时长限制在两小时内。
	lifetime := token.ExpiresIn
	if lifetime <= 0 || lifetime > 7200 {
		lifetime = 7200
	}
	return token.AccessToken, time.Now().Add(time.Duration(lifetime) * time.Second), nil
}

// checkServiceIdentity 通过 GetUserBasicInfo API 检查 MCP Access Token 是否健康可用。
func checkServiceIdentity(ctx context.Context, client *Client, token string) error {
	info, err := client.GetUserBasicInfo(ctx, token)
	if err != nil {
		return err
	}
	if info.UserId != "00001" || info.Name != "李建中" || info.UserTypeName != "教职工" {
		return fmt.Errorf("unexpected MCP service identity")
	}
	return nil
}
