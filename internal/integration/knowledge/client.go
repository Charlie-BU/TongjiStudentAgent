package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/volcengine/volc-sdk-golang/base"
)

const (
	defaultDomain = "api-knowledgebase.mlp.cn-beijing.volces.com"
	searchPath    = "/api/knowledge/collection/search_knowledge"
)

// Client 用于为主 Agent 检索 Ark 知识库。
type Client struct {
	ak         string
	sk         string
	domain     string
	collection string
	project    string
	resourceID string
	limit      int
	httpClient *http.Client
}

// NewFromEnv 根据环境变量创建知识库客户端。
func NewFromEnv() (*Client, error) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_ENABLED")))
	if err != nil && os.Getenv("ARK_KNOWLEDGE_ENABLED") != "" {
		return nil, fmt.Errorf("parse ARK_KNOWLEDGE_ENABLED: %w", err)
	}
	if !enabled {
		return nil, nil
	}

	client := &Client{
		ak:         strings.TrimSpace(os.Getenv("ARK_AK")),
		sk:         strings.TrimSpace(os.Getenv("ARK_SK")),
		domain:     envOrDefault("ARK_KNOWLEDGE_DOMAIN", defaultDomain),
		collection: strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_COLLECTION")),
		project:    envOrDefault("ARK_KNOWLEDGE_PROJECT", "default"),
		resourceID: strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_RESOURCE_ID")),
		limit:      5,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	if client.ak == "" || client.sk == "" {
		return nil, fmt.Errorf("ARK_AK and ARK_SK must be set when knowledge is enabled")
	}
	if client.collection == "" && client.resourceID == "" {
		return nil, fmt.Errorf("ARK_KNOWLEDGE_COLLECTION or ARK_KNOWLEDGE_RESOURCE_ID must be set when knowledge is enabled")
	}
	if value := strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_LIMIT")); value != "" {
		client.limit, err = strconv.Atoi(value)
		if err != nil || client.limit <= 0 {
			return nil, fmt.Errorf("ARK_KNOWLEDGE_LIMIT must be a positive integer")
		}
	}
	return client, nil
}

// Search 检索与 query 最相关的知识库切片。
func (c *Client) Search(ctx context.Context, query string) (*SearchKnowledgeRes, error) {
	requestBody, err := json.Marshal(SearchKnowledgeReq{
		Name:        c.collection,
		Project:     c.project,
		ResourceID:  c.resourceID,
		Query:       query,
		Limit:       c.limit,
		DenseWeight: 0.5,
		PreProcessing: &PreProcessing{
			NeedInstruction: true,
		},
		PostProcessing: &PostProcessing{
			RetrieveCount: c.limit,
			ChunkGroup:    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal knowledge search request: %w", err)
	}

	req, err := c.newRequest(ctx, requestBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send knowledge search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read knowledge search response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("knowledge search returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result SearchKnowledgeRes
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal knowledge search response: %w", err)
	}
	if result.Code != 0 {
		return &result, fmt.Errorf("knowledge search returned code %d: %s", result.Code, result.Message)
	}
	return &result, nil
}

// newRequest 创建知识库检索请求
func (c *Client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	u := url.URL{Scheme: "https", Host: c.domain, Path: searchPath}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create knowledge search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", c.domain)
	return base.Credentials{
		AccessKeyID:     c.ak,
		SecretAccessKey: c.sk,
		Service:         "air",
		Region:          "cn-north-1",
	}.Sign(req), nil
}

func envOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
