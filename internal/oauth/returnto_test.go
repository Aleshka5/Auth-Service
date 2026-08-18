package oauth

import (
	"strings"
	"testing"
)

func TestResolveReturnTo(t *testing.T) {
	allowed := map[string][]string{
		"filenkov.store":         {"name"},
		"storage.filenkov.store": {"name"},
	}

	ok, err := ResolveReturnTo("https://storage.filenkov.store/app", "https://filenkov.store", allowed, false)
	if err != nil || ok != "https://storage.filenkov.store/app" {
		t.Fatalf("got %q %v", ok, err)
	}

	if _, err := ResolveReturnTo("https://evil.example/x", "https://filenkov.store", allowed, false); err != ErrNotWhitelisted {
		t.Fatalf("evil host: %v", err)
	}
	if _, err := ResolveReturnTo("http://filenkov.store/", "https://filenkov.store", allowed, false); err != ErrNotWhitelisted {
		t.Fatalf("http prod host: %v", err)
	}
	if _, err := ResolveReturnTo("https://notfilenkov.store/", "https://filenkov.store", allowed, false); err != ErrNotWhitelisted {
		t.Fatalf("suffix host: %v", err)
	}

	local, err := ResolveReturnTo("http://localhost:5173/auth", "http://localhost", allowed, true)
	if err != nil || !strings.HasPrefix(local, "http://localhost:5173") {
		t.Fatalf("localhost: %q %v", local, err)
	}
	if _, err := ResolveReturnTo("http://localhost:5173/", "http://localhost", allowed, false); err != ErrNotWhitelisted {
		t.Fatalf("localhost without dev flag: %v", err)
	}

	def, err := ResolveReturnTo("", "https://filenkov.store", allowed, false)
	if err != nil || def != "https://filenkov.store" {
		t.Fatalf("default: %q %v", def, err)
	}
	if _, err := ResolveReturnTo("javascript:alert(1)", "https://filenkov.store", allowed, false); err != ErrInvalidReturnTo {
		t.Fatalf("javascript: %v", err)
	}
	if _, err := ResolveReturnTo("https://user:pass@filenkov.store/", "https://filenkov.store", allowed, false); err != ErrInvalidReturnTo {
		t.Fatalf("userinfo: %v", err)
	}
	stripped, err := ResolveReturnTo("https://filenkov.store/x#frag", "https://filenkov.store", allowed, false)
	if err != nil || strings.Contains(stripped, "#") {
		t.Fatalf("fragment: %q %v", stripped, err)
	}
	loop, err := ResolveReturnTo("https://127.0.0.1/ok", "http://localhost", allowed, true)
	if err != nil || loop != "https://127.0.0.1/ok" {
		t.Fatalf("https loopback: %q %v", loop, err)
	}
}
