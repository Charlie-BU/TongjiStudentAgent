// Package chat 提供当前聊天应用服务。
package chat

import "context"

var defaultService *Service

// Init 从环境变量初始化默认 Chat 服务。
func Init(ctx context.Context) error {
	service, err := NewFromEnv(ctx)
	if err != nil {
		return err
	}
	defaultService = service
	return nil
}

// NewFromEnv 从环境变量构造 Chat 服务。
func NewFromEnv(ctx context.Context) (*Service, error) {
	return defaultInitializationDeps().initialize(ctx)
}

// Close 释放默认聊天服务持有的资源。
func Close() error {
	if defaultService == nil {
		return nil
	}
	return defaultService.Close()
}

// Close 释放聊天服务持有的外部资源。
func (s *Service) Close() error {
	if s == nil || s.closeResources == nil {
		return nil
	}
	return s.closeResources()
}

// DefaultService 返回启动时初始化的服务，用于绑定 HTTP Handler。
func DefaultService() *Service { return defaultService }
