package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr string
	RedisURL string

	UserServiceURL string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	CookieDomain string
	CookieSecure bool

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	DevHTTP      bool
	PublicOrigin string

	GatewayHosts  map[string][]string
	GatewayConfig string

	// Owl chat. The API talks to the LiteLLM proxy. That proxy is not the
	// public host llm.filenkov.store; LLMHost is its OpenAI-compatible base URL.
	LLMHost   string
	LLMAPIKey string
	LLMModel  string
	// "none" keeps a reasoning model from thinking before a two-sentence
	// answer. Set LLM_REASONING_EFFORT to "" if the proxy rejects the field.
	LLMReasoningEffort string

	// MLflow tracing. Unset endpoint disables it; the chat is unaffected.
	MLflowEndpoint     string
	MLflowExperimentID string
}

// LoadShared reads Redis/cookie/JWT env used by both the API and the gateway.
// It does not require Google OAuth secrets.
func LoadShared() (*Config, error) {
	devHTTP := envBool("AUTH_DEV_HTTP", false)

	accessTTL, err := parseDuration(env("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	refreshTTL, err := parseDuration(env("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET is required and must be at least 32 bytes")
	}

	cfg := &Config{
		HTTPAddr:       env("HTTP_ADDR", ":8000"),
		RedisURL:       env("REDIS_URL", "redis://127.0.0.1:6379/0"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		CookieSecure:   !devHTTP,
		JWTSecret:      secret,
		JWTAccessTTL:   accessTTL,
		JWTRefreshTTL:  refreshTTL,
		DevHTTP:        devHTTP,
		PublicOrigin:   strings.TrimRight(env("PUBLIC_ORIGIN", "https://filenkov.store"), "/"),
		GatewayConfig:  strings.TrimSpace(os.Getenv("GATEWAY_CONFIG")),
		UserServiceURL: strings.TrimRight(env("USER_SERVICE_URL", "http://user_service:8000"), "/"),
	}

	if cfg.CookieDomain == "" && !devHTTP {
		cfg.CookieDomain = ".filenkov.store"
	}
	return cfg, nil
}

func Load() (*Config, error) {
	cfg, err := LoadShared()
	if err != nil {
		return nil, err
	}

	cfg.LLMHost = strings.TrimRight(env("LLM_HOST", "http://litellm-proxy-server:4000/v1"), "/")
	cfg.LLMAPIKey = env("LLM_API_KEY", "dummy")
	cfg.LLMModel = env("LLM_MODEL", "gpt-4o-mini")
	cfg.LLMReasoningEffort = strings.TrimSpace(envOr("LLM_REASONING_EFFORT", "none"))
	cfg.MLflowEndpoint = strings.TrimRight(strings.TrimSpace(os.Getenv("MLFLOW_TRACING_ENDPOINT")), "/")
	cfg.MLflowExperimentID = strings.TrimSpace(os.Getenv("MLFLOW_EXPERIMENT_ID"))

	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	cfg.GoogleRedirectURL = os.Getenv("GOOGLE_REDIRECT_URL")

	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.GoogleRedirectURL == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL are required")
	}
	if cfg.UserServiceURL == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL is required")
	}
	return cfg, nil
}

// SetGatewayHosts replaces the OAuth return_to allowlist with gateway YAML hosts.
func (c *Config) SetGatewayHosts(hosts []string) {
	m := make(map[string][]string, len(hosts)+1)
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		m[h] = []string{}
	}
	if c.PublicOrigin != "" {
		if origin := hostFromOrigin(c.PublicOrigin); origin != "" {
			m[origin] = []string{}
		}
	}
	c.GatewayHosts = m
}

// ReturnToAllowlist is the host map passed to oauth.ResolveReturnTo.
// Gateway YAML hosts win when SetGatewayHosts was called; otherwise the public origin
// (and loopback in AUTH_DEV_HTTP) is the allowlist.
func (c *Config) ReturnToAllowlist() map[string][]string {
	if c == nil {
		return nil
	}
	if len(c.GatewayHosts) > 0 {
		return c.GatewayHosts
	}
	m := map[string][]string{}
	if origin := hostFromOrigin(c.PublicOrigin); origin != "" {
		m[origin] = []string{}
	}
	if c.DevHTTP {
		m["localhost"] = []string{}
		m["127.0.0.1"] = []string{}
	}
	return m
}

// JWTAudiences is the access/refresh JWT aud list: hub host, YAML hosts, and loopback in dev.
func (c *Config) JWTAudiences() []string {
	if c == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	add(hostFromOrigin(c.PublicOrigin))
	for h := range c.GatewayHosts {
		add(h)
	}
	if c.DevHTTP {
		add("localhost")
		add("127.0.0.1")
	}
	if len(out) == 0 {
		add("filenkov.store")
	}
	return out
}

func hostFromOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	if !strings.Contains(origin, "://") {
		origin = "https://" + origin
	}
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// envOr differs from env: a variable that is set but empty stays empty, so a
// field can be switched off rather than falling back to its default.
func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
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
