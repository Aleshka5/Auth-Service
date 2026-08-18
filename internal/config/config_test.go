package config

import "testing"

func TestLoadDevHTTP(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost/oauth/google/callback")
	t.Setenv("AUTH_DEV_HTTP", "true")
	t.Setenv("COOKIE_DOMAIN", "")
	t.Setenv("PUBLIC_ORIGIN", "http://localhost")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure {
		t.Fatal("cookie must not be Secure when AUTH_DEV_HTTP")
	}
	if cfg.CookieDomain != "" {
		t.Fatalf("expected empty cookie domain, got %q", cfg.CookieDomain)
	}
	if _, ok := cfg.Whitelist["localhost"]; !ok {
		t.Fatal("localhost should be a whitelist host in dev")
	}
	if _, ok := cfg.Whitelist["storage.filenkov.store"]; !ok {
		t.Fatal("storage host missing")
	}
	if _, ok := cfg.Whitelist["llm.filenkov.store"]; ok {
		t.Fatal("llm must not be a validate audience")
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_REDIRECT_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestWhitelistJSONMerge(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "https://filenkov.store/oauth/google/callback")
	t.Setenv("WHITELIST_JSON", `{"resume.filenkov.store":["name"]}`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Whitelist["resume.filenkov.store"]; len(got) != 1 || got[0] != "name" {
		t.Fatalf("%v", got)
	}
	if _, ok := cfg.Whitelist["storage.filenkov.store"]; !ok {
		t.Fatal("defaults should remain")
	}
}

func TestDefaultWhitelist(t *testing.T) {
	wl := DefaultWhitelist()
	if len(wl["filenkov.store"]) != 3 {
		t.Fatalf("hub fields: %v", wl["filenkov.store"])
	}
}
