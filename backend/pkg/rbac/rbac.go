// Package rbac 提供角色权限码表与判断，独立于 middleware 避免包环。
package rbac

import "github.com/Fanc1er/Kate/backend/internal/master/models"

// PermissionsOf 返回某角色拥有的权限码集合（供 /auth/me 返回与按钮级权限）。
func PermissionsOf(role string) []string {
	switch role {
	case models.RoleAdmin:
		return append(AllPermissions(), "platform:read")
	case models.RoleUser:
		return userPermissions()
	default:
		return nil
	}
}

// userPermissions 普通用户业务权限集（不含用户管理与平台管理）。
func userPermissions() []string {
	return []string{
		"asset:read", "asset:export", "asset:write", "asset:batch-delete",
		"wechat:write", "policy:write", "plan:write", "task:write", "task:delete",
		"event:write", "noise:write", "alert:write", "vuln:write", "ticket:write",
		"evidence:upload", "report:delete", "report-template:write",
		"worker:write", "channel:write", "route:write", "rules:write", "intel-sub:write",
		"whitelist:write", "token:write", "webhook:write", "scenario:write",
		"escalation:write", "watch:write",
	}
}

// AllPermissions 所有权限码（admin 全集）。
func AllPermissions() []string {
	return []string{
		"asset:read", "asset:export", "asset:write", "asset:batch-delete",
		"wechat:write", "policy:write", "plan:write", "task:write", "task:delete",
		"event:write", "noise:write", "alert:write", "vuln:write", "ticket:write",
		"evidence:upload", "report:delete", "report-template:write", "member:write",
		"worker:write", "channel:write", "route:write", "rules:write", "intel-sub:write",
		"whitelist:write", "token:write", "webhook:write", "scenario:write",
		"escalation:write", "watch:write",
	}
}

// MatchPermission 判断角色是否拥有某权限码。
func MatchPermission(role, perm string) bool {
	for _, p := range PermissionsOf(role) {
		if p == perm {
			return true
		}
	}
	return false
}
