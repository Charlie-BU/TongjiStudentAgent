package webtool

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// PublicHost 校验公开域名或全局单播 IP 的基本形式。
func PublicHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip) && !netip.MustParsePrefix("2001:db8::/32").Contains(ip)
	}
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range []string{"localhost", "local", "internal", "lan", "home", "test", "invalid"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return false
		}
	}
	labels := strings.Split(host, ".")
	// 拒绝数字式及十六进制式主机名，避免旧式 IP 表示歧义。
	if _, err := strconv.ParseUint(labels[len(labels)-1], 0, 64); err == nil {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// PublicURL 校验公开 URL 并移除片段，不解析远端 DNS 或抓取页面。
func PublicURL(raw string) (string, error) {
	if len(raw) > 2048 {
		return "", errors.New("url not allowed")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || !PublicHost(u.Hostname()) {
		return "", errors.New("url not allowed")
	}
	if u.Port() != "" && u.Port() != "80" && u.Port() != "443" {
		return "", errors.New("url not allowed")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", errors.New("url not allowed")
	}
	for key := range query {
		key = strings.ToLower(key)
		for _, sensitive := range []string{"token", "secret", "password", "signature", "credential", "api_key", "apikey"} {
			if strings.Contains(key, sensitive) {
				return "", errors.New("url not allowed")
			}
		}
		if key == "code" || key == "authorization" || key == "sig" {
			return "", errors.New("url not allowed")
		}
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

// Domains 规范化只含域名的过滤条件。
func Domains(values []string) ([]string, error) {
	if len(values) > 10 {
		return nil, errors.New("too many domains")
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
		if !PublicHost(value) || strings.ContainsAny(value, "/:@?#") {
			return nil, errors.New("invalid domain")
		}
		if _, err := netip.ParseAddr(value); err == nil {
			return nil, errors.New("expected domain")
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}
