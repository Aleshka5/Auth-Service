package users

import (
	"context"
	"testing"
)

func TestMemoryAdminBootstrapOnce(t *testing.T) {
	ctx := context.Background()
	m := NewMemory("admin@filenkov.store")

	u, inserted, err := m.UpsertFromGoogle(ctx, GoogleProfile{Sub: "sub-1", Email: "admin@filenkov.store", Name: "A"})
	if err != nil || !inserted || u.Roles != RoleAdmin {
		t.Fatalf("first insert: inserted=%v roles=%s err=%v", inserted, u.Roles, err)
	}

	_ = m.Put(User{
		GoogleSub: "other-admin", GoogleEmail: "other@filenkov.store", Name: "O",
		Roles: RoleAdmin, StorageRoles: ServiceRoleStranger, ResumeRoles: ServiceRoleStranger,
	})
	stranger := RoleStranger
	if _, err := m.UpdateRoles(ctx, u.ID, RolePatch{Roles: &stranger}); err != nil {
		t.Fatal(err)
	}

	again, inserted, err := m.UpsertFromGoogle(ctx, GoogleProfile{Sub: "sub-1", Email: "admin@filenkov.store", Name: "A2"})
	if err != nil || inserted {
		t.Fatalf("update: inserted=%v err=%v", inserted, err)
	}
	if again.Roles != RoleStranger {
		t.Fatalf("must not re-force ADMIN on later login, got %s", again.Roles)
	}
	if again.Name != "A2" {
		t.Fatalf("name: %s", again.Name)
	}
}

func TestMemoryDefaultsStranger(t *testing.T) {
	ctx := context.Background()
	m := NewMemory("admin@filenkov.store")
	u, _, err := m.UpsertFromGoogle(ctx, GoogleProfile{Sub: "s", Email: "x@y.z", Name: ""})
	if err != nil {
		t.Fatal(err)
	}
	if u.Roles != RoleStranger || u.StorageRoles != ServiceRoleStranger || u.ResumeRoles != ServiceRoleStranger {
		t.Fatalf("defaults: %+v", u)
	}
	if u.Name != "x@y.z" {
		t.Fatalf("name fallback: %s", u.Name)
	}
}

func TestMemoryLastAdmin(t *testing.T) {
	ctx := context.Background()
	m := NewMemory("")
	admin := m.Put(User{GoogleSub: "a", GoogleEmail: "a@x", Name: "A", Roles: RoleAdmin, StorageRoles: ServiceRoleStranger, ResumeRoles: ServiceRoleStranger})
	other := m.Put(User{GoogleSub: "b", GoogleEmail: "b@x", Name: "B", Roles: RoleStranger, StorageRoles: ServiceRoleFamily, ResumeRoles: ServiceRoleStranger})

	stranger := RoleStranger
	if _, err := m.UpdateRoles(ctx, admin.ID, RolePatch{Roles: &stranger}); err != ErrLastAdmin {
		t.Fatalf("demote last admin: %v", err)
	}
	if _, err := m.SetBlocked(ctx, admin.ID, true); err != ErrLastAdmin {
		t.Fatalf("block last admin: %v", err)
	}

	adminRole := RoleAdmin
	if _, err := m.UpdateRoles(ctx, other.ID, RolePatch{Roles: &adminRole}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetBlocked(ctx, admin.ID, true); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryNotFound(t *testing.T) {
	ctx := context.Background()
	m := NewMemory("")
	if _, err := m.GetByID(ctx, [16]byte{}); err != ErrNotFound {
		t.Fatalf("get: %v", err)
	}
}
