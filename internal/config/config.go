package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	DatabaseURL    string
	RedisURL       string
	MigrationsPath string

	AdminEmail string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	CookieDomain string
	CookieName   string
	CookieSecure bool
	SessionTTL   time.Duration

	DevHTTP      bool
	PublicOrigin string

	Whitelist map[string][]string
}

func Load() (*Config, error) {
	ttl, err := parseDuration(env("SESSION_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("SESSION_TTL: %w", err)
	}

	devHTTP := envBool("AUTH_DEV_HTTP", false)

	cfg := &Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		GRPCAddr:           env("GRPC_ADDR", ":9090"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           env("REDIS_URL", "redis://127.0.0.1:6379/0"),
		MigrationsPath:     env("MIGRATIONS_PATH", "migrations"),
		AdminEmail:         strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		CookieDomain:       os.Getenv("COOKIE_DOMAIN"),
		CookieName:         env("COOKIE_NAME", "auth_session"),
		CookieSecure:       !devHTTP,
		SessionTTL:         ttl,
		DevHTTP:            devHTTP,
		PublicOrigin:       strings.TrimRight(env("PUBLIC_ORIGIN", "https://filenkov.store"), "/"),
	}

	if cfg.CookieDomain == "" && !devHTTP {
		cfg.CookieDomain = ".filenkov.store"
	}

	wl, err := parseWhitelist(os.Getenv("WHITELIST_JSON"))
	if err != nil {
		return nil, err
	}
	if cfg.DevHTTP {
		hub := append([]string{}, wl["filenkov.store"]...)
		if len(hub) == 0 {
			hub = []string{"google_email", "name", "roles"}
		}
		wl["localhost"] = hub
		wl["127.0.0.1"] = hub
	}
	cfg.Whitelist = wl

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.GoogleRedirectURL == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL are required")
	}
	if cfg.AdminEmail == "" {
		return nil, fmt.Errorf("ADMIN_EMAIL is required")
	}
	return cfg, nil
}

func DefaultWhitelist() map[string][]string {
	return map[string][]string{
		"filenkov.store":         {"google_email", "name", "roles"},
		"storage.filenkov.store": {"id", "google_email", "name", "storage_roles"},
		"resume.filenkov.store":  {"name", "resume_roles"},
	}
}

func parseWhitelist(raw string) (map[string][]string, error) {
	wl := DefaultWhitelist()
	if strings.TrimSpace(raw) == "" {
		return wl, nil
	}
	var extra map[string][]string
	if err := json.Unmarshal([]byte(raw), &extra); err != nil {
		return nil, fmt.Errorf("WHITELIST_JSON: %w", err)
	}
	for host, fields := range extra {
		wl[strings.ToLower(strings.TrimSpace(host))] = fields
	}
	return wl, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func parseDuration(v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return d, nil
}
