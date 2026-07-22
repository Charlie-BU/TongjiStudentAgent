// Package tongjiapi 提供同济开放平台 OAuth 2.0 和数据接口客户端。
package tongjiapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultAuthorizationEndpoint = "https://api.tongji.edu.cn/keycloak/realms/OpenPlatform/protocol/openid-connect/auth"
	defaultTokenEndpoint         = "https://api.tongji.edu.cn/v1/token"
	defaultAPIBaseURL            = "https://api.tongji.edu.cn"
	defaultTimeout               = 10 * time.Second
	stateLifetime                = 10 * time.Minute
)

// Config 保存同济开放平台客户端配置。
type Config struct {
	ClientID              string
	ClientSecret          string
	RedirectURI           string
	StateSecret           string
	AuthorizationEndpoint string
	TokenEndpoint         string
	APIBaseURL            string
	Timeout               time.Duration
}

// TokenResponse 表示开放平台返回的 OAuth 令牌。
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	IDToken          string `json:"id_token"`
	Scope            string `json:"scope"`
}

// APIResponse 表示开放平台数据接口的通用响应体。
type APIResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

// Client 调用同济开放平台 OAuth 和受保护数据接口。
type Client struct {
	config     Config
	httpClient *http.Client
}

// statePayload 保存签名 state 中的最小防篡改信息。
type statePayload struct {
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"expires_at"`
}

// NewFromEnv 从 TONGJI_OPEN_PLATFORM_* 环境变量创建客户端。
func NewFromEnv() (*Client, error) {
	timeout := defaultTimeout

	return New(Config{
		ClientID:              strings.TrimSpace(os.Getenv("TONGJI_OPEN_PLATFORM_CLIENT_ID")),
		ClientSecret:          strings.TrimSpace(os.Getenv("TONGJI_OPEN_PLATFORM_CLIENT_SECRET")),
		RedirectURI:           strings.TrimSpace(os.Getenv("TONGJI_OPEN_PLATFORM_REDIRECT_URI")),
		StateSecret:           strings.TrimSpace(os.Getenv("TONGJI_OPEN_PLATFORM_STATE_SECRET")),
		AuthorizationEndpoint: envOrDefault("TONGJI_OPEN_PLATFORM_AUTHORIZATION_ENDPOINT", defaultAuthorizationEndpoint),
		TokenEndpoint:         envOrDefault("TONGJI_OPEN_PLATFORM_TOKEN_ENDPOINT", defaultTokenEndpoint),
		APIBaseURL:            strings.TrimRight(envOrDefault("TONGJI_OPEN_PLATFORM_API_BASE_URL", defaultAPIBaseURL), "/"),
		Timeout:               timeout,
	})
}

// New 使用显式配置创建客户端。
func New(config Config) (*Client, error) {
	if config.ClientID == "" || config.ClientSecret == "" || config.RedirectURI == "" || config.StateSecret == "" {
		return nil, fmt.Errorf("client ID, client secret, redirect URI and state secret are required")
	}
	if config.AuthorizationEndpoint == "" || config.TokenEndpoint == "" || config.APIBaseURL == "" {
		return nil, fmt.Errorf("Open Platform endpoints are required")
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	return &Client{config: config, httpClient: &http.Client{Timeout: config.Timeout}}, nil
}

// CreateState 创建带有效期的签名 OAuth state。
func (c *Client) CreateState() (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate OAuth state nonce: %w", err)
	}
	payload, err := json.Marshal(statePayload{
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
		ExpiresAt: time.Now().Add(stateLifetime).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal OAuth state: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return encodedPayload + "." + c.signState(encodedPayload), nil
}

// ValidateState 验证 OAuth state 的签名和有效期。
func (c *Client) ValidateState(state string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("OAuth state is malformed")
	}
	if !hmac.Equal([]byte(parts[1]), []byte(c.signState(parts[0]))) {
		return fmt.Errorf("OAuth state signature is invalid")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("decode OAuth state: %w", err)
	}
	var payload statePayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return fmt.Errorf("unmarshal OAuth state: %w", err)
	}
	if payload.Nonce == "" || payload.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("OAuth state is expired")
	}
	return nil
}

// AuthorizationURL 返回授权码模式的浏览器重定向地址。
func (c *Client) AuthorizationURL(state string, scopes []string) (string, error) {
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("OAuth state is required")
	}
	authorizationURL, err := url.Parse(c.config.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.config.ClientID)
	query.Set("redirect_uri", c.config.RedirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("kc_idp_hint", "tjiam")
	authorizationURL.RawQuery = query.Encode()
	return authorizationURL.String(), nil
}

// ExchangeAuthorizationCode 使用授权码交换 OAuth 令牌。
func (c *Client) ExchangeAuthorizationCode(ctx context.Context, code string) (*TokenResponse, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("authorization code is required")
	}
	return c.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURI},
	})
}

// RefreshAccessToken 使用 refresh token 更新 OAuth 令牌。
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("refresh token is required")
	}
	return c.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"refresh_token": {refreshToken},
	})
}

// GetSingleInfo 调用个人基础信息接口。
func (c *Client) GetSingleInfo(ctx context.Context, accessToken string) (*APIResponse, error) {
	return c.Get(ctx, "/v1/dc/user/single_info", accessToken)
}

// Get 调用同源开放平台 GET 数据接口。
func (c *Client) Get(ctx context.Context, resourcePath, accessToken string) (*APIResponse, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("access token is required")
	}
	if !strings.HasPrefix(resourcePath, "/") {
		return nil, fmt.Errorf("resource path must start with /")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.APIBaseURL+resourcePath, nil)
	if err != nil {
		return nil, fmt.Errorf("create Open Platform request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return c.doAPIRequest(req)
}

// signState 为 state 载荷生成 HMAC-SHA256 签名。
func (c *Client) signState(encodedPayload string) string {
	mac := hmac.New(sha256.New, []byte(c.config.StateSecret))
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// requestToken 发送 application/x-www-form-urlencoded OAuth 令牌请求。
func (c *Client) requestToken(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.TokenEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Open Platform token endpoint returned HTTP %d", resp.StatusCode)
	}
	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("Open Platform token response did not include access_token")
	}
	return &token, nil
}

// doAPIRequest 解析开放平台数据接口响应。
func (c *Client) doAPIRequest(req *http.Request) (*APIResponse, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send Open Platform request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Open Platform response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Open Platform returned HTTP %d", resp.StatusCode)
	}
	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal Open Platform response: %w", err)
	}
	if result.Code != "A00000" {
		return nil, fmt.Errorf("Open Platform returned code %s: %s", result.Code, result.Message)
	}
	return &result, nil
}

// envOrDefault 返回非空环境变量或默认值。
func envOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
