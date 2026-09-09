package tavily

import "errors"

// Error 表示不包含上游正文或凭据的稳定服务错误。
type Error struct {
	Status     string
	HTTPStatus int
}

// Error 返回安全错误标识。
func (e *Error) Error() string { return e.Status }

// Status 返回供工具使用的稳定错误分类。
func Status(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Status
	}
	return "web_unavailable"
}

// statusForHTTP 将供应商状态码转换为稳定分类。
func statusForHTTP(code int) string {
	switch code {
	case 400:
		return "invalid_arguments"
	case 429:
		return "rate_limited"
	case 432, 433:
		return "quota_exceeded"
	default:
		return "web_unavailable"
	}
}
