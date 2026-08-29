package whitelist

import (
	"context"
	"testing"

	"github.com/aleshka5/auth-service/internal/users"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestProject(t *testing.T) {
	l := New(map[string][]string{
		"filenkov.store":         {"google_email", "name", "roles"},
		"storage.filenkov.store": {"id", "google_email", "name", "storage_roles"},
		"resume.filenkov.store":  {"name", "resume_roles"},
	})
	u := &users.User{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		GoogleEmail:  "a@b.c",
		Name:         "Ann",
		Roles:        users.RoleAdmin,
		StorageRoles: users.ServiceRoleFamily,
		ResumeRoles:  users.ServiceRoleStranger,
	}

	hub, err := l.Project("https://filenkov.store", u)
	if err != nil {
		t.Fatal(err)
	}
	if hub["google_email"] != "a@b.c" || hub["roles"] != "ADMIN" || hub["storage_roles"] != "" || hub["id"] != "" {
		t.Fatalf("hub projection leaked extra fields: %#v", hub)
	}

	st, err := l.Project("storage.filenkov.store:443", u)
	if err != nil {
		t.Fatal(err)
	}
	if st["id"] != u.ID.String() || st["google_email"] != "a@b.c" || st["name"] != "Ann" || st["storage_roles"] != "FAMILY" {
		t.Fatalf("storage projection: %#v", st)
	}
	if _, ok := st["roles"]; ok {
		t.Fatalf("storage projection leaked hub roles: %#v", st)
	}

	rs, err := l.Project("resume.filenkov.store", u)
	if err != nil {
		t.Fatal(err)
	}
	if rs["name"] != "Ann" || rs["resume_roles"] != "STRANGER" || rs["id"] != "" || rs["google_email"] != "" || rs["storage_roles"] != "" {
		t.Fatalf("resume projection: %#v", rs)
	}

	if _, err := l.Project("llm.filenkov.store", u); err != ErrUnknownHost {
		t.Fatalf("llm should not be an audience: %v", err)
	}
}

func TestProjectUnknownField(t *testing.T) {
	l := New(map[string][]string{"filenkov.store": {"uuid"}})
	if _, err := l.Project("filenkov.store", &users.User{}); err == nil {
		t.Fatal("expected unknown field")
	}
}

func TestPersist(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	l := New(map[string][]string{"storage.filenkov.store": {"name"}})
	if err := l.Persist(context.Background(), rdb); err != nil {
		t.Fatal(err)
	}
	if got, _ := mr.Get("whitelist"); got == "" {
		t.Fatal("expected whitelist key")
	}
}

func TestHasHostExact(t *testing.T) {
	l := New(map[string][]string{"filenkov.store": {"name"}})
	if l.HasHost("evil.filenkov.store") {
		t.Fatal("suffix must not match")
	}
	if !l.HasHost("filenkov.store") {
		t.Fatal("exact host should match")
	}
}
