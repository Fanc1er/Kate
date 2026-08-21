package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// MD5Hex 计算字符串 MD5 十六进制摘要（用于 URL/证据去重）。
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SHA256Hex 计算字节数据 SHA-256 十六进制摘要。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SHA256HexString 计算字符串 SHA-256 摘要。
func SHA256HexString(s string) string {
	return SHA256Hex([]byte(s))
}

// NormalizeURL 强制标准化 URL（协议/默认端口/路径/去 query 片段），
// 对齐 R4.4-1「资产 URL 入库前强制标准化」。
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	// 默认端口省略
	if u.Port() == "80" && u.Scheme == "http" {
		u.Host = u.Hostname()
	}
	if u.Port() == "443" && u.Scheme == "https" {
		u.Host = u.Hostname()
	}
	// 仅保留路径部分，去除查询与片段；路径为空补 "/"
	path := strings.TrimRight(u.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = path
	return u.String(), nil
}

// URLKey 计算归一化 URL 的 MD5 去重键。
func URLKey(normalized string) string {
	return MD5Hex(normalized)
}

// Truncate 截断字符串至指定字节数。
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
