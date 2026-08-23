package knowledge

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	vikingknowledge "github.com/volcengine/vikingdb-go-sdk/knowledge"
	"github.com/volcengine/vikingdb-go-sdk/knowledge/model"
)

const defaultEndpoint = "https://api-knowledgebase.mlp.cn-beijing.volces.com"

type collectionClient interface {
	SearchKnowledge(context.Context, model.SearchKnowledgeRequest, ...vikingknowledge.RequestOption) (*model.SearchKnowledgeResponse, error)
	ListDocs(context.Context, model.ListDocsRequest, ...vikingknowledge.RequestOption) (*model.ListDocsResponse, error)
}

// Client 用于访问 Ark 知识库。
type Client struct {
	collection collectionClient
	limit      int
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

	apiKey := strings.TrimSpace(os.Getenv("VOLC_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("VOLC_API_KEY must be set when knowledge is enabled")
	}
	collectionName := strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_COLLECTION"))
	resourceID := strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_RESOURCE_ID"))
	if collectionName == "" && resourceID == "" {
		return nil, fmt.Errorf("ARK_KNOWLEDGE_COLLECTION or ARK_KNOWLEDGE_RESOURCE_ID must be set when knowledge is enabled")
	}

	limit := 5
	if value := strings.TrimSpace(os.Getenv("ARK_KNOWLEDGE_LIMIT")); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("ARK_KNOWLEDGE_LIMIT must be a positive integer")
		}
	}
	endpoint, err := normalizeEndpoint(envOrDefault("ARK_KNOWLEDGE_DOMAIN", defaultEndpoint))
	if err != nil {
		return nil, err
	}
	client, err := vikingknowledge.New(
		vikingknowledge.AuthAPIKey(apiKey),
		vikingknowledge.WithEndpoint(endpoint),
		vikingknowledge.WithRegion(envOrDefault("ARK_KNOWLEDGE_REGION", "cn-beijing")),
		vikingknowledge.WithTimeout(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("create VikingDB knowledge client: %w", err)
	}
	return &Client{collection: client.Collection(model.CollectionMeta{
		CollectionName: collectionName,
		ProjectName:    envOrDefault("ARK_KNOWLEDGE_PROJECT", "default"),
		ResourceID:     resourceID,
	}), limit: limit}, nil
}

func normalizeEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid ARK_KNOWLEDGE_DOMAIN: %q", endpoint)
	}
	return strings.TrimSuffix(endpoint, "/"), nil
}

func envOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
