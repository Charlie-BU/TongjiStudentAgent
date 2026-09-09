package webtool

import "github.com/Charlie-BU/TongjiStudent/internal/platform/publicurl"

// PublicHost 校验公开域名或全局单播 IP 的基本形式。
func PublicHost(host string) bool { return publicurl.PublicHost(host) }

// PublicURL 校验公开 URL 并移除片段，不解析远端 DNS 或抓取页面。
func PublicURL(raw string) (string, error) { return publicurl.PublicURL(raw) }

// Domains 规范化只含域名的过滤条件。
func Domains(values []string) ([]string, error) { return publicurl.Domains(values) }
