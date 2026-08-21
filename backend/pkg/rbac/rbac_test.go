package rbac

import (
	"strings"
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// 表驱动：四角色 × 关键读写权限，断言 viewer 无写权限、super_admin 仅平台权限。
func TestPermissionsMatrix(t *testing.T) {
	cases := []struct {
		name string
		role string
		perm string
		want bool
	}{
		// org_admin 拥有全部权限码。
		{"org_admin-asset-write", models.RoleOrgAdmin, "asset:write", true},
		{"org_admin-member-write", models.RoleOrgAdmin, "member:write", true},
		{"org_admin-task-delete", models.RoleOrgAdmin, "task:delete", true},
		{"org_admin-worker-write", models.RoleOrgAdmin, "worker:write", true},
		{"org_admin-evidence-upload", models.RoleOrgAdmin, "evidence:upload", true},
		{"org_admin-platform-read", models.RoleOrgAdmin, "platform:read", false},
		// engineer 拥有资产/任务/处置写权限，但无成员与策略写权限。
		{"engineer-asset-write", models.RoleEngineer, "asset:write", true},
		{"engineer-task-write", models.RoleEngineer, "task:write", true},
		{"engineer-event-write", models.RoleEngineer, "event:write", true},
		{"engineer-evidence-upload", models.RoleEngineer, "evidence:upload", true},
		{"engineer-member-write", models.RoleEngineer, "member:write", false},
		{"engineer-policy-write", models.RoleEngineer, "policy:write", false},
		{"engineer-worker-write", models.RoleEngineer, "worker:write", false},
		// viewer 只读，任何写权限都禁止。
		{"viewer-asset-read", models.RoleViewer, "asset:read", true},
		{"viewer-asset-export", models.RoleViewer, "asset:export", true},
		{"viewer-asset-write", models.RoleViewer, "asset:write", false},
		{"viewer-event-write", models.RoleViewer, "event:write", false},
		{"viewer-task-write", models.RoleViewer, "task:write", false},
		{"viewer-alert-write", models.RoleViewer, "alert:write", false},
		{"viewer-evidence-upload", models.RoleViewer, "evidence:upload", false},
		// super_admin 平台通道，权限码含 org:write/platform:read。
		{"super-admin-org-write", models.RoleSuperAdmin, "org:write", true},
		{"super-admin-platform-read", models.RoleSuperAdmin, "platform:read", true},
		{"super-admin-asset-write", models.RoleSuperAdmin, "asset:write", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchPermission(c.role, c.perm); got != c.want {
				t.Fatalf("MatchPermission(%q, %q) = %v, want %v", c.role, c.perm, got, c.want)
			}
		})
	}
}

// viewer 的全部权限码集合不得包含任何 *:write。
func TestViewerNoWritePermissions(t *testing.T) {
	for _, p := range PermissionsOf(models.RoleViewer) {
		if strings.HasSuffix(p, ":write") {
			t.Fatalf("viewer 不应拥有写权限 %q", p)
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
