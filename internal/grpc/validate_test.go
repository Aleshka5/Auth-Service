package grpcapi

import (
	"context"
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/session"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/aleshka5/auth-service/internal/whitelist"
	"github.com/aleshka5/auth-service/pkg/authpb"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testValidateServer(t *testing.T) (*Server, *users.Memory, *session.Store) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	mem := users.NewMemory("")
	sess := session.NewStore(rdb, time.Hour)
	wl := whitelist.New(config.DefaultWhitelist())
	return New(sess, mem, wl), mem, sess
}

func TestValidateProjectsStorageFields(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	u := mem.Put(users.User{
		GoogleSub: "s", GoogleEmail: "secret@x", Name: "Ann",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleFamily, ResumeRoles: users.ServiceRoleStranger,
	})
	sid, err := sess.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Validate(ctx, &authpb.ValidateRequest{
		SessionId:  sid,
		CallerHost: "https://storage.filenkov.store/files",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Fields["id"] != u.ID.String() {
		t.Fatalf("id: %v", resp.Fields)
	}
	if resp.Fields["google_email"] != "secret@x" || resp.Fields["name"] != "Ann" || resp.Fields["storage_roles"] != "FAMILY" {
		t.Fatalf("%v", resp.Fields)
	}
	if _, ok := resp.Fields["roles"]; ok {
		t.Fatalf("leaked hub roles: %v", resp.Fields)
	}
}

func TestValidateDenied(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	u := mem.Put(users.User{
		GoogleSub: "s", GoogleEmail: "a@x", Name: "A",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	sid, _ := sess.Create(ctx, u.ID)

	cases := []struct {
		name string
		req  *authpb.ValidateRequest
		code codes.Code
	}{
		{"missing", &authpb.ValidateRequest{}, codes.InvalidArgument},
		{"bad session", &authpb.ValidateRequest{SessionId: "nope", CallerHost: "filenkov.store"}, codes.Unauthenticated},
		{"unknown host", &authpb.ValidateRequest{SessionId: sid, CallerHost: "llm.filenkov.store"}, codes.PermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.Validate(ctx, tc.req)
			if status.Code(err) != tc.code {
				t.Fatalf("got %v want %v", err, tc.code)
			}
		})
	}

	blocked := mem.Put(users.User{
		GoogleSub: "b", GoogleEmail: "b@x", Name: "B",
		Roles: users.RoleBlocked, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	bsid, _ := sess.Create(ctx, blocked.ID)
	_, err := srv.Validate(ctx, &authpb.ValidateRequest{SessionId: bsid, CallerHost: "filenkov.store"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("blocked: %v", err)
	}

	storageBlocked := mem.Put(users.User{
		GoogleSub: "sb", GoogleEmail: "sb@x", Name: "SB",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleBlocked, ResumeRoles: users.ServiceRoleFamily,
	})
	ssid, _ := sess.Create(ctx, storageBlocked.ID)
	_, err = srv.Validate(ctx, &authpb.ValidateRequest{SessionId: ssid, CallerHost: "storage.filenkov.store"})
	if status.Code(err) != codes.PermissionDenied || status.Convert(err).Message() != "blocked" {
		t.Fatalf("storage blocked: %v", err)
	}
	resp, err := srv.Validate(ctx, &authpb.ValidateRequest{SessionId: ssid, CallerHost: "resume.filenkov.store"})
	if err != nil {
		t.Fatalf("resume must still work: %v", err)
	}
	if resp.Fields["resume_roles"] != "FAMILY" {
		t.Fatalf("resume fields: %v", resp.Fields)
	}
}

func TestListUsersAdminOK(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	admin := mem.Put(users.User{
		GoogleSub: "admin", GoogleEmail: "admin@x", Name: "Admin",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleAdmin, ResumeRoles: users.ServiceRoleStranger,
	})
	other := mem.Put(users.User{
		GoogleSub: "other", GoogleEmail: "other@x", Name: "Other",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleFamily, ResumeRoles: users.ServiceRoleStranger,
	})
	sid, err := sess.Create(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  sid,
		CallerHost: "storage.filenkov.store",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Users) != 2 {
		t.Fatalf("want 2 users, got %d", len(resp.Users))
	}
	byID := map[string]map[string]string{}
	for _, u := range resp.Users {
		id := u.Fields["id"]
		if id == "" {
			t.Fatalf("missing id: %v", u.Fields)
		}
		if u.Fields["storage_roles"] == "" {
			t.Fatalf("missing storage_roles: %v", u.Fields)
		}
		if _, ok := u.Fields["session_id"]; ok {
			t.Fatalf("must not return session ids: %v", u.Fields)
		}
		byID[id] = u.Fields
	}
	if byID[admin.ID.String()]["storage_roles"] != "ADMIN" {
		t.Fatalf("admin row: %v", byID[admin.ID.String()])
	}
	if byID[other.ID.String()]["storage_roles"] != "FAMILY" || byID[other.ID.String()]["google_email"] != "other@x" {
		t.Fatalf("other row: %v", byID[other.ID.String()])
	}
}

func TestListUsersFamilyDenied(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	u := mem.Put(users.User{
		GoogleSub: "f", GoogleEmail: "f@x", Name: "Fam",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleFamily, ResumeRoles: users.ServiceRoleAdmin,
	})
	sid, _ := sess.Create(ctx, u.ID)
	_, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  sid,
		CallerHost: "storage.filenkov.store",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestListUsersBadSession(t *testing.T) {
	ctx := context.Background()
	srv, _, _ := testValidateServer(t)
	_, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  "expired",
		CallerHost: "storage.filenkov.store",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("got %v", err)
	}
}

func TestListUsersBlockedCaller(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	u := mem.Put(users.User{
		GoogleSub: "b", GoogleEmail: "b@x", Name: "Blocked",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleBlocked, ResumeRoles: users.ServiceRoleAdmin,
	})
	sid, _ := sess.Create(ctx, u.ID)
	_, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  sid,
		CallerHost: "storage.filenkov.store",
	})
	if status.Code(err) != codes.PermissionDenied || status.Convert(err).Message() != "blocked" {
		t.Fatalf("got %v", err)
	}
}

func TestListUsersHubBlockedStillStorageAdmin(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	u := mem.Put(users.User{
		GoogleSub: "b", GoogleEmail: "b@x", Name: "HubBlocked",
		Roles: users.RoleBlocked, StorageRoles: users.ServiceRoleAdmin, ResumeRoles: users.ServiceRoleStranger,
	})
	sid, _ := sess.Create(ctx, u.ID)
	resp, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  sid,
		CallerHost: "storage.filenkov.store",
	})
	if err != nil {
		t.Fatalf("hub block must not deny storage admin: %v", err)
	}
	if len(resp.Users) != 1 {
		t.Fatalf("want 1 user, got %d", len(resp.Users))
	}
}

func TestListUsersReflectsUpdateRoles(t *testing.T) {
	ctx := context.Background()
	srv, mem, sess := testValidateServer(t)
	admin := mem.Put(users.User{
		GoogleSub: "admin", GoogleEmail: "admin@x", Name: "Admin",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleAdmin, ResumeRoles: users.ServiceRoleStranger,
	})
	target := mem.Put(users.User{
		GoogleSub: "target", GoogleEmail: "t@x", Name: "Target",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleFamily, ResumeRoles: users.ServiceRoleStranger,
	})
	sid, err := sess.Create(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	next := users.ServiceRoleStranger
	if _, err := mem.UpdateRoles(ctx, target.ID, users.RolePatch{StorageRoles: &next}); err != nil {
		t.Fatal(err)
	}

	resp, err := srv.ListUsers(ctx, &authpb.ListUsersRequest{
		SessionId:  sid,
		CallerHost: "storage.filenkov.store",
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range resp.Users {
		if u.Fields["id"] == target.ID.String() {
			found = true
			if u.Fields["storage_roles"] != "STRANGER" {
				t.Fatalf("want STRANGER after UpdateRoles, got %v", u.Fields)
			}
		}
	}
	if !found {
		t.Fatal("target user missing from ListUsers")
	}
}
