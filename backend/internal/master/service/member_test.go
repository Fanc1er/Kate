package service

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

func TestInviteRejectsSuperAdmin(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	org := models.Organization{Name: "org", Status: models.StatusActive, MaxMembers: 10}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	sa := models.User{Username: "sa", Email: "sa@example.com", IsSuperAdmin: true}
	if err := gdb.Create(&sa).Error; err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	_, err := s.Invite(org.ID, sa.Email, models.RoleViewer, 1, "op", "127.0.0.1", "test")
	if err == nil {
		t.Fatal("expected error inviting super admin")
	}
	if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
	var count int64
	gdb.Model(&models.UserOrg{}).Where("org_id = ?", org.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected no user_orgs record, got %d", count)
	}
}

func TestInviteNormalUserOK(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	org := models.Organization{Name: "org", Status: models.StatusActive, MaxMembers: 10}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	u := models.User{Username: "u", Email: "u@example.com"}
	if err := gdb.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	uo, err := s.Invite(org.ID, u.Email, models.RoleEngineer, 1, "op", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if uo.Role != models.RoleEngineer {
		t.Fatalf("expected engineer role, got %s", uo.Role)
	}
}

func TestCreateOrgRejectsSuperAdmin(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	sa := models.User{Username: "sa", Email: "sa@example.com", IsSuperAdmin: true}
	if err := gdb.Create(&sa).Error; err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	_, err := s.CreateOrg(sa.ID, "org", "free", "sa", "127.0.0.1", "test")
	if err == nil {
		t.Fatal("expected error creating org for super admin")
	}
	if errs.CodeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("expected CodeValidationFailed, got %v", err)
	}
	var orgCount int64
	gdb.Model(&models.Organization{}).Count(&orgCount)
	if orgCount != 0 {
		t.Fatalf("expected no org created, got %d", orgCount)
	}
}

func TestCreateOrgNormalUserOK(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMemberService(gdb, nil, nil)

	u := models.User{Username: "u", Email: "u@example.com"}
	if err := gdb.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	org, err := s.CreateOrg(u.ID, "org", "free", "u", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	var uo models.UserOrg
	if err := gdb.Where("user_id = ? AND org_id = ?", u.ID, org.ID).First(&uo).Error; err != nil {
		t.Fatalf("expected owner user_orgs record: %v", err)
	}
	if uo.Role != models.RoleOrgAdmin {
		t.Fatalf("expected owner as org_admin, got %s", uo.Role)
	}
}
