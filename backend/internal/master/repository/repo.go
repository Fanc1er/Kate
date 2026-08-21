// Package repository 提供 org_id 强制过滤守卫与通用数据访问辅助。
package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// Guard 组织隔离守卫，缺省 org_id 拒绝查询（对应关键不变量「租户隔离」）。
type Guard struct {
	DB    *gorm.DB
	OrgID int64
}

// NewGuard 构造守卫。
func NewGuard(db *gorm.DB, orgID int64) (*Guard, error) {
	if orgID < 0 {
		return nil, errs.New(errs.CodeOrgRequired, "")
	}
	return &Guard{DB: db, OrgID: orgID}, nil
}

// OrgID 返回当前组织 ID。
func (g *Guard) OrgIDValue() int64 { return g.OrgID }

// Scoped 为业务模型查询附加 org_id 过滤。
func (g *Guard) Scoped(model any) *gorm.DB {
	return g.DB.Model(model).Where("org_id = ?", g.OrgID)
}

// ScopedAll 附加 org_id 过滤且不限制模型（super_admin org_id=0 时不加过滤）。
func (g *Guard) ScopedAll() *gorm.DB {
	if g.OrgID == 0 {
		return g.DB
	}
	return g.DB.Where("org_id = ?", g.OrgID)
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
