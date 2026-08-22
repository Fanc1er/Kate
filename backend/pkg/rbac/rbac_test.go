package rbac

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// 表驱动：admin/user 两角色 × 关键读写权限，断言 user 无成员与平台管理权限。
func TestPermissionsMatrix(t *testing.T) {
	cases := []struct {
		name string
		role string
		perm string
		want bool
	}{
		// admin 拥有全部权限码 + 平台权限。
		{"admin-asset-write", models.RoleAdmin, "asset:write", true},
		{"admin-member-write", models.RoleAdmin, "member:write", true},
		{"admin-task-delete", models.RoleAdmin, "task:delete", true},
		{"admin-worker-write", models.RoleAdmin, "worker:write", true},
		{"admin-evidence-upload", models.RoleAdmin, "evidence:upload", true},
		{"admin-platform-read", models.RoleAdmin, "platform:read", true},
		{"admin-org-write", models.RoleAdmin, "org:write", false},
		// user 拥有业务读写权限，但无成员与平台管理权限。
		{"user-asset-write", models.RoleUser, "asset:write", true},
		{"user-task-write", models.RoleUser, "task:write", true},
		{"user-task-delete", models.RoleUser, "task:delete", true},
		{"user-event-write", models.RoleUser, "event:write", true},
		{"user-evidence-upload", models.RoleUser, "evidence:upload", true},
		{"user-policy-write", models.RoleUser, "policy:write", true},
		{"user-worker-write", models.RoleUser, "worker:write", true},
		{"user-member-write", models.RoleUser, "member:write", false},
		{"user-platform-read", models.RoleUser, "platform:read", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchPermission(c.role, c.perm); got != c.want {
				t.Fatalf("MatchPermission(%q, %q) = %v, want %v", c.role, c.perm, got, c.want)
			}
		})
	}
}

// user 的权限码集合不得包含任何 member:write 或 platform:read。
func TestUserNoMemberOrPlatformPermissions(t *testing.T) {
	for _, p := range PermissionsOf(models.RoleUser) {
		if p == "member:write" || p == "platform:read" {
			t.Fatalf("user 不应拥有权限码 %q", p)
		}
	}
}

// 未知角色权限集为空。
func TestUnknownRoleEmpty(t *testing.T) {
	if got := PermissionsOf("ghost"); len(got) != 0 {
		t.Fatalf("未知角色权限集应为空，得到 %v", got)
	}
}

// 权限码集合无重复。
func TestNoDuplicatePermissions(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllPermissions() {
		if seen[p] {
			t.Fatalf("重复权限码 %q", p)
		}
		seen[p] = true
	}
}
