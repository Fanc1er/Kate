// Package repository 提供通用数据访问辅助（单租户，无组织隔离）。
package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Guard 通用查询辅助，封装分页、过滤、排序等数据访问工具。
type Guard struct {
	DB *gorm.DB
}

// NewGuard 构造查询辅助。
func NewGuard(db *gorm.DB) *Guard {
	return &Guard{DB: db}
}

// Scoped 为业务模型返回模型查询（单租户，无组织过滤）。
func (g *Guard) Scoped(model any) *gorm.DB {
	return g.DB.Model(model)
}

// ScopedAll 返回基础查询（单租户，无组织过滤）。
func (g *Guard) ScopedAll() *gorm.DB {
	return g.DB
}

// ApplyFilter 应用 filter[字段]=值 查询参数（白名单字段）。
func (g *Guard) ApplyFilter(q *gorm.DB, params map[string]string, allowed map[string]string) *gorm.DB {
	for k, v := range params {
		col, ok := allowed[k]
		if !ok || v == "" {
			continue
		}
		q = q.Where(col+" = ?", v)
	}
	return q
}

// ApplySort 应用 sort=field,-field 排序参数。
func ApplySort(q *gorm.DB, sort string, allowed map[string]string) *gorm.DB {
	if sort == "" {
		return q.Order("id DESC")
	}
	fields := strings.Split(sort, ",")
	for _, f := range fields {
		f = strings.TrimSpace(f)
		desc := false
		if strings.HasPrefix(f, "-") {
			desc = true
			f = f[1:]
		}
		col, ok := allowed[f]
		if !ok {
			continue
		}
		if desc {
			q = q.Order(col + " DESC")
		} else {
			q = q.Order(col + " ASC")
		}
	}
	return q
}

// CountAndPage 执行分页计数并返回记录。
func CountAndPage[T any](q *gorm.DB, page, pageSize int) (list []T, total int64, err error) {
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// IsNotFound 判断是否为记录不存在。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
