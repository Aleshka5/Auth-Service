package grpcapi

import (
	"context"
	"testing"
	"time"

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
	wl := whitelist.New(map[string][]string{
		"filenkov.store":         {"google_email", "name", "roles"},
		"storage.filenkov.store": {"name", "storage_roles"},
		"resume.filenkov.store":  {"name", "resume_roles"},
	})
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
	if resp.Fields["name"] != "Ann" || resp.Fields["storage_roles"] != "FAMILY" {
		t.Fatalf("%v", resp.Fields)
	}
	if _, ok := resp.Fields["google_email"]; ok {
		t.Fatalf("leaked email: %v", resp.Fields)
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
		GoogleSub: "b", GoogleEmail: "b@x", Name: "B", Blocked: true,
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	bsid, _ := sess.Create(ctx, blocked.ID)
	_, err := srv.Validate(ctx, &authpb.ValidateRequest{SessionId: bsid, CallerHost: "filenkov.store"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("blocked: %v", err)
	}
}
