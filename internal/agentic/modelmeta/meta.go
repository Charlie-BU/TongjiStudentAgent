// Package modelmeta 定义模型协议元数据与会话路由上下文。
package modelmeta

import "context"

// ProtocolKey 标识消息中需要持久化的模型协议数据。
const ProtocolKey = "model-protocol-data"

// WithSession 将会话标识绑定到本轮请求。
func WithSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope{SessionID: id})
}

// SessionID 返回本轮模型请求的会话标识。
func SessionID(ctx context.Context) string {
	s, _ := ctx.Value(scopeKey{}).(scope)
	return s.SessionID
}
