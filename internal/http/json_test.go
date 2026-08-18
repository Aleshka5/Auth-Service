package httpapi

import (
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/users"
	"github.com/google/uuid"
)

func TestToUserJSONNullLastSeen(t *testing.T) {
	u := &users.User{
		ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		GoogleEmail:  "x@y.z",
		Name:         "X",
		Roles:        users.RoleStranger,
		StorageRoles: users.ServiceRoleStranger,
		ResumeRoles:  users.ServiceRoleStranger,
	}
	j := toUserJSON(u)
	if j.LastSeenAt != nil {
		t.Fatalf("expected null last_seen_at, got %v", *j.LastSeenAt)
	}
	ts := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	u.LastSeenAt = &ts
	j = toUserJSON(u)
	if j.LastSeenAt == nil || *j.LastSeenAt != "2026-08-13T12:00:00Z" {
		t.Fatalf("last_seen_at: %v", j.LastSeenAt)
	}
}
