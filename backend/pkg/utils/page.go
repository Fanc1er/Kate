// Package utils 提供通用工具函数：分页、URL 归一化、Hash、时间等。
package utils

import "strconv"

// ParsePage 解析 page 参数，默认 1。
func ParsePage(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// ParsePageSize 解析 page_size 参数，默认 20，上限 200。
func ParsePageSize(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 20
	}
	if n > 200 {
		return 200
	}
	return n
}

// Offset 计算分页偏移量（page 从 1 起）。
func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}

// ParseInt64 解析 int64，非法时返回 0。
func ParseInt64(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
