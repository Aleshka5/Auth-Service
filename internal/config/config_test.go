package config

import "testing"

const testJWTSecret = "test-jwt-secret-key-32-bytes-ok!"

func TestLoadSharedSkipsGoogle(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_REDIRECT_URL", "")
	t.Setenv("AUTH_DEV_HTTP", "true")
	t.Setenv("COOKIE_DOMAIN", "")
	cfg, err := LoadShared()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure {
		t.Fatal("cookie must not be Secure when AUTH_DEV_HTTP")
	}
	if cfg.GoogleClientID != "" {
		t.Fatal("shared load must not require Google")
	}
	if cfg.HTTPAddr != ":8000" {
		t.Fatalf("HTTP_ADDR default %q", cfg.HTTPAddr)
	}
	if cfg.JWTAccessTTL.Minutes() != 15 {
		t.Fatalf("access ttl %v", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL.Hours() != 168 {
		t.Fatalf("refresh ttl %v", cfg.JWTRefreshTTL)
	}
}

func TestLoadSharedRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	if _, err := LoadShared(); err == nil {
		t.Fatal("expected error")
	}
	t.Setenv("JWT_SECRET", "")
	if _, err := LoadShared(); err == nil {
		t.Fatal("expected error")
	}
}

func TestReturnToAllowlistGatewayHosts(t *testing.T) {
	cfg := &Config{
		PublicOrigin: "https://filenkov.store",
	}
	al := cfg.ReturnToAllowlist()
	if _, ok := al["filenkov.store"]; !ok {
		t.Fatal("public origin host is the fallback allowlist")
	}
	if _, ok := al["storage.filenkov.store"]; ok {
		t.Fatal("storage must not be implied without gateway YAML")
	}
	cfg.SetGatewayHosts([]string{"filenkov.store"})
	al = cfg.ReturnToAllowlist()
	if _, ok := al["filenkov.store"]; !ok {
		t.Fatal("hub missing")
	}
	if _, ok := al["storage.filenkov.store"]; ok {
		t.Fatal("gateway YAML is source of truth; storage must not leak")
	}
}

func TestJWTAudiences(t *testing.T) {
	cfg := &Config{
		PublicOrigin: "https://filenkov.store",
		DevHTTP:      true,
	}
	aud := cfg.JWTAudiences()
	want := map[string]bool{"filenkov.store": true, "localhost": true, "127.0.0.1": true}
	for _, a := range aud {
		if !want[a] {
			t.Fatalf("unexpected aud %q in %v", a, aud)
		}
		delete(want, a)
	}
	if len(want) != 0 {
		t.Fatalf("missing %v", want)
	}
	cfg.SetGatewayHosts([]string{"filenkov.store", "storage.filenkov.store"})
	aud = cfg.JWTAudiences()
	foundStorage := false
	for _, a := range aud {
		if a == "storage.filenkov.store" {
			foundStorage = true
		}
	}
	if !foundStorage {
		t.Fatalf("storage host must be in JWT aud when listed in YAML hosts, got %v", aud)
	}
}

func TestLoadDevHTTP(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost/oauth/google/callback")
	t.Setenv("AUTH_DEV_HTTP", "true")
	t.Setenv("COOKIE_DOMAIN", "")
	t.Setenv("PUBLIC_ORIGIN", "http://localhost")
	t.Setenv("USER_SERVICE_URL", "http://user_service:8000")

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
	if cfg.UserServiceURL != "http://user_service:8000" {
		t.Fatalf("USER_SERVICE_URL %q", cfg.UserServiceURL)
	}
	aud := cfg.JWTAudiences()
	found := map[string]bool{}
	for _, a := range aud {
		found[a] = true
	}
	if !found["localhost"] || !found["127.0.0.1"] {
		t.Fatalf("dev audiences %v", aud)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_REDIRECT_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadReasoningEffortDefaultsToNone(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "https://filenkov.store/oauth/google/callback")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMReasoningEffort != "none" {
		t.Fatalf("reasoning effort %q", cfg.LLMReasoningEffort)
	}
	if cfg.LLMHost != "http://litellm-proxy-server:4000/v1" || cfg.LLMAPIKey != "dummy" {
		t.Fatalf("llm defaults %q %q", cfg.LLMHost, cfg.LLMAPIKey)
	}

	// Set-but-empty switches the field off rather than restoring the default.
	t.Setenv("LLM_REASONING_EFFORT", "")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMReasoningEffort != "" {
		t.Fatalf("empty must stay empty, got %q", cfg.LLMReasoningEffort)
	}
}
