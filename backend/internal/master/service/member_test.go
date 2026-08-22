package service

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

func TestInviteCreatesInvitedUser(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	u, err := s.Invite("new@example.com", models.RoleUser, 1, "op", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if u.Status != models.StatusInvited || u.Role != models.RoleUser || u.Username != "new@example.com" {
		t.Fatalf("invited user = %+v", u)
	}
	var count int64
	gdb.Model(&models.User{}).Count(&count)
	if count != 1 {
		t.Fatalf("应创建 1 个用户, got %d", count)
	}
}

func TestInviteUpdatesExistingUserRole(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)
	existing := models.User{Username: "u", Email: "u@example.com", Role: models.RoleUser, Status: models.StatusActive}
	if err := gdb.Create(&existing).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	u, err := s.Invite("u@example.com", models.RoleAdmin, 1, "op", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if u.ID != existing.ID || u.Role != models.RoleAdmin {
		t.Fatalf("应更新既有用户角色, got %+v", u)
	}
}

func TestInviteInvalidRole(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	if _, err := s.Invite("x@example.com", "ghost", 1, "op", "127.0.0.1", "test"); err == nil {
		t.Fatal("非法角色应拒绝")
	} else if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
}

func TestUpdateRoleLastAdmin(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)
	admin := models.User{Username: "sa", Email: "sa@example.com", Role: models.RoleAdmin}
	if err := gdb.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if err := s.UpdateRole(admin.ID, models.RoleUser, 1, "op", "127.0.0.1", "test"); err == nil {
		t.Fatal("不能降级最后一个管理员")
	} else if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
}

func TestToggleStatus(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)
	u := models.User{Username: "u", Email: "u@example.com", Status: models.StatusActive}
	if err := gdb.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := s.ToggleStatus(u.ID, models.StatusDisabled, 1, "op", "127.0.0.1", "test"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	var got models.User
	gdb.First(&got, u.ID)
	if got.Status != models.StatusDisabled {
		t.Fatalf("status = %s, want disabled", got.Status)
	}
	if err := s.ToggleStatus(u.ID, models.StatusActive, 1, "op", "127.0.0.1", "test"); err != nil {
		t.Fatalf("enable: %v", err)
	}
}

func TestRemoveSelf(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	if err := s.Remove(7, 7, "op", "127.0.0.1", "test"); err == nil {
		t.Fatal("不能删除自己")
	} else if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
}

func TestRemoveLastAdmin(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)
	admin := models.User{Username: "sa", Email: "sa@example.com", Role: models.RoleAdmin}
	if err := gdb.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if err := s.Remove(admin.ID, 1, "op", "127.0.0.1", "test"); err == nil {
		t.Fatal("不能删除最后一个管理员")
	} else if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
}

func TestListMembers(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)
	for _, name := range []string{"a", "b"} {
		if err := gdb.Create(&models.User{Username: name}).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	list, total, err := s.List(1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("total=%d list=%d, want 2/2", total, len(list))
	}
}
