package gateway

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseValidates(t *testing.T) {
	_, err := Parse([]byte(`
upstreams: {api: "http://api:8000"}
hosts: {}
`))
	if err == nil {
		t.Fatal("expected error for empty hosts")
	}

	_, err = Parse([]byte(`
upstreams: {api: "http://api:8000"}
hosts:
  filenkov.store:
    role_field: llm_roles
    routes:
      - match: /
        upstream: api
        auth: public
`))
	if err == nil || !strings.Contains(err.Error(), "role_field") {
		t.Fatalf("unknown role_field: %v", err)
	}

	_, err = Parse([]byte(`
upstreams: {api: "http://api:8000"}
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /
        upstream: missing
        auth: public
`))
	if err == nil || !strings.Contains(err.Error(), "unknown upstream") {
		t.Fatalf("unknown upstream: %v", err)
	}

	_, err = Parse([]byte(`
upstreams: {api: "http://api:8000"}
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /
        upstream: api
        auth: everyone
`))
	if err == nil || !strings.Contains(err.Error(), "unknown auth") {
		t.Fatalf("unknown auth: %v", err)
	}
}

func TestParseDefaults(t *testing.T) {
	f, err := Parse([]byte(`
upstreams:
  web: "http://web:80"
hosts:
  filenkov.store:
    routes:
      - match: /*
        upstream: web
        auth: public
`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Listen != ":8083" {
		t.Fatalf("listen %q", f.Listen)
	}
	if f.Timeouts.ReadHeader != 15*time.Second || f.Timeouts.Idle != 120*time.Second || f.Timeouts.Proxy != 60*time.Second {
		t.Fatalf("timeouts %+v", f.Timeouts)
	}
	if f.RateLimit.RequestsPerMinute != 180 {
		t.Fatalf("rpm %d", f.RateLimit.RequestsPerMinute)
	}
	r, err := f.Router()
	if err != nil {
		t.Fatal(err)
	}
	h, _, ok := r.Match("filenkov.store", "/")
	if !ok || h.roleField != "global_role" {
		t.Fatalf("default role_field: ok=%v field=%q", ok, h.roleField)
	}
}

func TestParseRejectRouteNoUpstream(t *testing.T) {
	f, err := Parse([]byte(`
upstreams:
  web: "http://web:80"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /users
        auth: reject
      - match: /*
        upstream: web
        auth: public
`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.Router()
	if err != nil {
		t.Fatal(err)
	}
	_, rt, ok := r.Match("filenkov.store", "/users")
	if !ok || rt.auth != PolicyReject {
		t.Fatalf("ok=%v %+v", ok, rt)
	}
}

func TestLoadDefaultYAML(t *testing.T) {
	path := filepath.Join("..", "..", "deploy", "gateway.yaml")
	f, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	names := f.Hostnames()
	found := false
	for _, n := range names {
		if n == "filenkov.store" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hosts %v", names)
	}
	foundStorage := false
	foundLLM := false
	for _, n := range names {
		if n == "storage.filenkov.store" {
			foundStorage = true
		}
		if n == "llm.filenkov.store" {
			foundLLM = true
		}
	}
	if !foundStorage {
		t.Fatal("storage.filenkov.store must be enabled")
	}
	if !foundLLM {
		t.Fatal("llm.filenkov.store must be enabled")
	}
	if got := f.Upstreams["api"]; got != "http://api:8000" {
		t.Fatalf("api upstream %q", got)
	}
	if got := f.Upstreams["user_service"]; got != "http://user_service:8000" {
		t.Fatalf("user_service upstream %q", got)
	}
	if got := f.Upstreams["storage"]; got != "http://app:8000" {
		t.Fatalf("storage upstream %q", got)
	}
	if got := f.Upstreams["hermes"]; got != "http://hermes:8787" {
		t.Fatalf("hermes upstream %q", got)
	}
	if got := f.Upstreams["litellm"]; got != "http://litellm-proxy-server:4000" {
		t.Fatalf("litellm upstream %q", got)
	}
	r, err := f.Router()
	if err != nil {
		t.Fatal(err)
	}
	h, rt, ok := r.Match("filenkov.store", "/hermes/x")
	if !ok || rt.auth != PolicySession || rt.stripPrefix != "/hermes" || rt.upstreamName != "hermes" || !rt.injectAccessJWT {
		t.Fatalf("hermes route: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.MatchMethod("filenkov.store", http.MethodGet, "/chat")
	if !ok || rt.redirect != "/hermes/chat" || rt.auth != PolicySession {
		t.Fatalf("chat redirect: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.MatchMethod("filenkov.store", http.MethodGet, "/sessions")
	if !ok || rt.redirect != "/hermes/sessions" || rt.auth != PolicySession {
		t.Fatalf("sessions redirect: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.MatchMethod("filenkov.store", http.MethodGet, "/hermes/login")
	if !ok || rt.redirect != "/hermes/sessions" || rt.auth != PolicySession {
		t.Fatalf("hermes login skip: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/hermes")
	if !ok || rt.auth != PolicySession || rt.upstreamName != "hermes" {
		t.Fatalf("hermes exact: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/hermes/")
	if !ok || rt.auth != PolicySession || rt.upstreamName != "hermes" {
		t.Fatalf("hermes slash: ok=%v %+v", ok, rt)
	}
	if h.roleField != "global_role" {
		t.Fatalf("hub role_field %q", h.roleField)
	}
	_, rt, ok = r.Match("filenkov.store", "/api/admin/users")
	if !ok || rt.auth != PolicySession || rt.match != "/api/*" {
		t.Fatalf("dropped /api/admin must fall through to /api/* session: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/api/me")
	if !ok || rt.auth != PolicySession {
		t.Fatalf("api me: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/api/refresh")
	if !ok || rt.auth != PolicyPublic || rt.match != "/api/refresh" {
		t.Fatalf("refresh: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/api/logout")
	if !ok || rt.auth != PolicyPublic {
		t.Fatalf("logout: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/users/by-google-sub/abc")
	if !ok || rt.auth != PolicyReject {
		t.Fatalf("by-google-sub: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.MatchMethod("filenkov.store", "POST", "/users")
	if !ok || rt.auth != PolicyReject {
		t.Fatalf("POST /users: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.MatchMethod("filenkov.store", "GET", "/users")
	if !ok || rt.auth != PolicyAdmin || rt.upstreamName != "user_service" || rt.match != "/users" {
		t.Fatalf("GET /users: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/users/3fa85f64-5717-4562-b3fc-2c963f66afa6")
	if !ok || rt.auth != PolicySession || rt.upstreamName != "user_service" {
		t.Fatalf("users uuid: ok=%v %+v", ok, rt)
	}
	_, rt, ok = r.Match("filenkov.store", "/")
	if !ok || rt.auth != PolicyPublic {
		t.Fatalf("home: ok=%v %+v", ok, rt)
	}
	h, rt, ok = r.Match("storage.filenkov.store", "/health")
	if !ok || rt.auth != PolicyPublic || rt.upstreamName != "storage" {
		t.Fatalf("storage health: ok=%v %+v", ok, rt)
	}
	h, rt, ok = r.Match("storage.filenkov.store", "/")
	if !ok || rt.auth != PolicySession || rt.upstreamName != "storage" {
		t.Fatalf("storage catch-all: ok=%v %+v", ok, rt)
	}
	if h.roleField != "storage" {
		t.Fatalf("storage role_field %q", h.roleField)
	}
	if h.proxyTimeout != 30*time.Minute {
		t.Fatalf("storage proxy timeout %v", h.proxyTimeout)
	}
	h, rt, ok = r.Match("llm.filenkov.store", "/")
	if !ok || rt.auth != PolicySession || rt.upstreamName != "litellm" {
		t.Fatalf("llm catch-all: ok=%v %+v", ok, rt)
	}
	if h.roleField != "global_role" {
		t.Fatalf("llm role_field %q", h.roleField)
	}
	if h.proxyTimeout != 10*time.Minute {
		t.Fatalf("llm proxy timeout %v", h.proxyTimeout)
	}
	m := f.AllowedHostsMap()
	if _, ok := m["storage.filenkov.store"]; !ok {
		t.Fatal("return_to allowlist must include storage.filenkov.store")
	}
	if _, ok := m["llm.filenkov.store"]; !ok {
		t.Fatal("return_to allowlist must include llm.filenkov.store")
	}
}

func TestParseHostProxyTimeout(t *testing.T) {
	f, err := Parse([]byte(`
upstreams:
  storage: "http://app:8000"
  web: "http://web:80"
hosts:
  filenkov.store:
    routes:
      - match: /*
        upstream: web
        auth: public
  storage.filenkov.store:
    role_field: storage
    timeouts:
      proxy: 30m
    routes:
      - match: /*
        upstream: storage
        auth: session
`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Timeouts.Proxy != 60*time.Second {
		t.Fatalf("default proxy %v", f.Timeouts.Proxy)
	}
	r, err := f.Router()
	if err != nil {
		t.Fatal(err)
	}
	h, _, ok := r.Match("storage.filenkov.store", "/")
	if !ok || h.proxyTimeout != 30*time.Minute {
		t.Fatalf("ok=%v timeout=%v", ok, h.proxyTimeout)
	}
	h, _, ok = r.Match("filenkov.store", "/")
	if !ok || h.proxyTimeout != 0 {
		t.Fatalf("hub must use file default, got %v", h.proxyTimeout)
	}
}

func TestParseHostProxyTimeoutInvalid(t *testing.T) {
	_, err := Parse([]byte(`
upstreams:
  web: "http://web:80"
hosts:
  filenkov.store:
    timeouts:
      proxy: -1s
    routes:
      - match: /*
        upstream: web
        auth: public
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := LoadFile(filepath.Join(t.TempDir(), "nope.yaml")); !os.IsNotExist(err) {
		t.Fatalf("got %v", err)
	}
}

func TestAllowedHostsMap(t *testing.T) {
	f, err := Parse([]byte(`
upstreams:
  web: "http://web:80"
hosts:
  filenkov.store:
    role_field: global_role
    aliases: [localhost]
    routes:
      - match: /*
        upstream: web
        auth: public
`))
	if err != nil {
		t.Fatal(err)
	}
	m := f.AllowedHostsMap()
	if _, ok := m["filenkov.store"]; !ok {
		t.Fatal("missing hub host")
	}
	if _, ok := m["localhost"]; !ok {
		t.Fatal("missing alias")
	}
	if _, ok := m["evil.example"]; ok {
		t.Fatal("unexpected host")
	}
}
