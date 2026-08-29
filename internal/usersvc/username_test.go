package usersvc

import "testing"

func TestDeriveUsername(t *testing.T) {
	cases := []struct {
		email, name, want string
	}{
		{"ada@example.com", "", "ada"},
		{"Ada@Example.com", "", "Ada"},
		{"ada+foo@gmail.com", "", "ada"},
		{"123@x.com", "", "u_123"},
		{"a@x.com", "", "axx"},
		{"+++@x.com", "Ada Lovelace", "Ada"},
		{"+++@x.com", "", "user"},
		{"...@x.com", "", "user"},
		{"ada.lovelace@x.com", "", "adalovelace"},
		{"110000000000000000001@x.com", "", "u_110000000000000000001"},
	}
	for _, tc := range cases {
		got := DeriveUsername(tc.email, tc.name)
		if got != tc.want {
			t.Errorf("DeriveUsername(%q, %q)=%q want %q", tc.email, tc.name, got, tc.want)
		}
		if !usernameRe.MatchString(got) {
			t.Errorf("DeriveUsername(%q) %q does not match username pattern", tc.email, got)
		}
	}
}

func TestUsernameAttemptCollision(t *testing.T) {
	base := DeriveUsername("ada@example.com", "")
	if base != "ada" {
		t.Fatalf("%q", base)
	}
	if got := UsernameAttempt(base, 1); got != "ada" {
		t.Fatalf("attempt 1 %q", got)
	}
	if got := UsernameAttempt(base, 2); got != "ada_2" {
		t.Fatalf("attempt 2 %q", got)
	}
	if got := UsernameAttempt(base, 3); got != "ada_3" {
		t.Fatalf("attempt 3 %q", got)
	}
}

func TestUsernameAttemptTruncatesLeft(t *testing.T) {
	long := stringsOf('a', 32)
	got := UsernameAttempt(long, 2)
	if len(got) != 32 {
		t.Fatalf("len %d %q", len(got), got)
	}
	if got[len(got)-2:] != "_2" {
		t.Fatalf("suffix lost: %q", got)
	}
	if !usernameRe.MatchString(got) {
		t.Fatalf("invalid %q", got)
	}
}

func TestNumericSubIsNotUsername(t *testing.T) {
	sub := "110000000000000000001"
	u := DeriveUsername("bob@example.com", "")
	if u == sub {
		t.Fatal("username must not be google_sub")
	}
	if u[0] >= '0' && u[0] <= '9' {
		t.Fatalf("username must start with a letter: %q", u)
	}
}

func stringsOf(r rune, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r)
	}
	return string(b)
}
