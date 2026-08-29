package users

import "testing"

func TestParseRole(t *testing.T) {
	r, err := ParseRole("admin")
	if err != nil || r != RoleAdmin {
		t.Fatalf("got %q %v", r, err)
	}
	blocked, err := ParseRole("blocked")
	if err != nil || blocked != RoleBlocked {
		t.Fatalf("got %q %v", blocked, err)
	}
	if _, err := ParseRole("USER"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseServiceRole(t *testing.T) {
	r, err := ParseServiceRole("FAMILY")
	if err != nil || r != ServiceRoleFamily {
		t.Fatalf("got %q %v", r, err)
	}
	blocked, err := ParseServiceRole("BLOCKED")
	if err != nil || blocked != ServiceRoleBlocked {
		t.Fatalf("got %q %v", blocked, err)
	}
	if _, err := ParseServiceRole("guest"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHasBlockedRole(t *testing.T) {
	if HasBlockedRole(map[string]string{"name": "Ann", "storage_roles": "FAMILY"}) {
		t.Fatal("family must not be blocked")
	}
	if !HasBlockedRole(map[string]string{"storage_roles": "BLOCKED", "name": "Ann"}) {
		t.Fatal("storage BLOCKED")
	}
	if !HasBlockedRole(map[string]string{"roles": "BLOCKED"}) {
		t.Fatal("hub BLOCKED")
	}
	if HasBlockedRole(map[string]string{"roles": "ADMIN", "storage_roles": "STRANGER"}) {
		t.Fatal("unrelated fields")
	}
}

func TestRemovesLastAdmin(t *testing.T) {
	admin := User{Roles: RoleAdmin}
	stranger := User{Roles: RoleStranger}
	blocked := RoleBlocked
	strangerRole := RoleStranger
	adminRole := RoleAdmin

	cases := []struct {
		name string
		n    int
		u    User
		next *Role
		want bool
	}{
		{"last admin demote", 1, admin, &strangerRole, true},
		{"last admin block", 1, admin, &blocked, true},
		{"two admins demote", 2, admin, &strangerRole, false},
		{"stranger to blocked", 1, stranger, &blocked, false},
		{"keep admin", 1, admin, &adminRole, false},
		{"no patch", 1, admin, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RemovesLastAdmin(tc.n, tc.u, tc.next)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
