package webfetch

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"

	"golang.org/x/net/html/charset"
)

// decodePage 校验响应类型并将受支持的文本解码为有界 UTF-8 内容。
func decodePage(raw []byte, contentType string) ([]byte, string, error) {
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(raw)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", &Error{Status: "fetch_failed"}
	}
	switch mediaType {
	case "text/html", "application/xhtml+xml", "text/plain":
	default:
		return nil, "", &Error{Status: "fetch_failed"}
	}
	if label := params["charset"]; label != "" {
		if encoding, _ := charset.Lookup(label); encoding == nil {
			return nil, "", &Error{Status: "fetch_failed"}
		}
	}
	// 依据 BOM、响应字符集和 HTML 声明统一解码正文与链接。
	reader, err := charset.NewReader(bytes.NewReader(raw), contentType)
	if err != nil {
		return nil, "", &Error{Status: "fetch_failed"}
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, maxPageBytes+1))
	if err != nil || len(decoded) > maxPageBytes {
		return nil, "", &Error{Status: "fetch_failed"}
	}
	return decoded, mediaType, nil
}
