package modelmeta

// scope 保存当前模型调用的会话标识。
type scope struct{ SessionID string }

// scopeKey 隔离模型请求上下文。
type scopeKey struct{}
