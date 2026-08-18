package users

import "testing"

func TestParseRole(t *testing.T) {
	r, err := ParseRole("admin")
	if err != nil || r != RoleAdmin {
		t.Fatalf("got %q %v", r, err)
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
	if _, err := ParseServiceRole("guest"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRemovesLastAdmin(t *testing.T) {
	admin := User{Roles: RoleAdmin}
	blockedAdmin := User{Roles: RoleAdmin, Blocked: true}
	stranger := User{Roles: RoleStranger}

	cases := []struct {
		name   string
		n      int
		u      User
		demote bool
		block  bool
		want   bool
	}{
		{"last admin demote", 1, admin, true, false, true},
		{"last admin block", 1, admin, false, true, true},
		{"two admins demote", 2, admin, true, false, false},
		{"stranger block", 1, stranger, false, true, false},
		{"already blocked", 1, blockedAdmin, true, true, false},
		{"no action", 1, admin, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RemovesLastAdmin(tc.n, tc.u, tc.demote, tc.block)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
