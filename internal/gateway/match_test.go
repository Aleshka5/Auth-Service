package gateway

import (
	"testing"
)

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/health", "/health", true},
		{"/health", "/healthz", false},
		{"/health", "/health/", false},
		{"/oauth/*", "/oauth", true},
		{"/oauth/*", "/oauth/", true},
		{"/oauth/*", "/oauth/google", true},
		{"/oauth/*", "/oauth/google/callback", true},
		{"/oauth/*", "/api/me", false},
		{"/oauth/*", "/oauthsneaky", false},
		{"/api/refresh", "/api/refresh", true},
		{"/api/refresh", "/api/refresh/", false},
		{"/api/*", "/api/me", true},
		{"/api/*", "/api/logout", true},
		{"/users/by-google-sub/*", "/users/by-google-sub/11001", true},
		{"/users", "/users", true},
		{"/users", "/users/x", false},
		{"/users/*", "/users", true},
		{"/users/*", "/users/abc/role", true},
		{"/hermes/*", "/hermes", true},
		{"/hermes/*", "/hermes/", true},
		{"/hermes/*", "/hermes/foo", true},
		{"/*", "/", true},
		{"/*", "/auth", true},
		{"/*", "/architecture/x", true},
		{"/admin", "/admin", true},
		{"/admin", "/admin/", false},
		{"/admin/*", "/admin/", true},
		{"/admin/*", "/admin/users", true},
	}
	for _, tc := range cases {
		if got := matchPath(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchPath(%q, %q)=%v want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestStripPath(t *testing.T) {
	cases := []struct {
		path, prefix, want string
	}{
		{"/hermes/foo", "/hermes", "/foo"},
		{"/hermes", "/hermes", "/"},
		{"/hermes/", "/hermes", "/"},
		{"/hermes/a/b", "/hermes", "/a/b"},
		{"/other", "/hermes", "/other"},
		{"/foo", "", "/foo"},
	}
	for _, tc := range cases {
		if got := stripPath(tc.path, tc.prefix); got != tc.want {
			t.Errorf("stripPath(%q, %q)=%q want %q", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestRouterFirstMatch(t *testing.T) {
	f, err := Parse([]byte(`
listen: ":0"
upstreams:
  api: "http://api:8000"
  web: "http://web:80"
  user_service: "http://user_service:8000"
hosts:
  filenkov.store:
    role_field: global_role
    aliases: [localhost]
    routes:
      - match: /api/refresh
        upstream: api
        auth: public
      - match: /api/*
        upstream: api
        auth: session
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users
        methods: [POST]
        auth: reject
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin
      - match: /users/*
        upstream: user_service
        auth: session
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

	cases := []struct {
		host, method, path, wantMatch, wantAuth, wantUp string
		ok                                              bool
	}{
		{"filenkov.store", "GET", "/api/refresh", "/api/refresh", PolicyPublic, "api", true},
		{"FILENKOV.STORE:443", "GET", "/api/me", "/api/*", PolicySession, "api", true},
		{"filenkov.store", "GET", "/users/by-google-sub/x", "/users/by-google-sub/*", PolicyReject, "", true},
		{"filenkov.store", "POST", "/users", "/users", PolicyReject, "", true},
		{"filenkov.store", "GET", "/users", "/users", PolicyAdmin, "user_service", true},
		{"filenkov.store", "PUT", "/users", "/users/*", PolicySession, "user_service", true},
		{"filenkov.store", "GET", "/users/", "/users/*", PolicySession, "user_service", true},
		{"filenkov.store", "PUT", "/users/abc/role", "/users/*", PolicySession, "user_service", true},
		{"localhost", "GET", "/", "/*", PolicyPublic, "web", true},
		{"evil.example", "GET", "/", "", "", "", false},
	}
	for _, tc := range cases {
		h, rt, ok := r.MatchMethod(tc.host, tc.method, tc.path)
		if ok != tc.ok {
			t.Errorf("%s %s %s ok=%v want %v", tc.host, tc.method, tc.path, ok, tc.ok)
			continue
		}
		if !tc.ok {
			continue
		}
		if h == nil || rt == nil {
			t.Fatalf("%s %s: nil match", tc.host, tc.path)
		}
		if rt.match != tc.wantMatch || rt.auth != tc.wantAuth || rt.upstreamName != tc.wantUp {
			t.Errorf("%s %s %s: match=%q auth=%q up=%q", tc.host, tc.method, tc.path, rt.match, rt.auth, rt.upstreamName)
		}
		if h.roleField != "global_role" {
			t.Errorf("role_field %q", h.roleField)
		}
	}
}

func TestRouterStripCompiled(t *testing.T) {
	f, err := Parse([]byte(`
upstreams:
  hermes: "http://host.docker.internal:8787"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /hermes/*
        upstream: hermes
        auth: admin
        strip_prefix: /hermes
`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.Router()
	if err != nil {
		t.Fatal(err)
	}
	_, rt, ok := r.Match("filenkov.store", "/hermes/tools")
	if !ok || rt.stripPrefix != "/hermes" {
		t.Fatalf("ok=%v strip=%q", ok, rt.stripPrefix)
	}
	if got := stripPath("/hermes/tools", rt.stripPrefix); got != "/tools" {
		t.Fatalf("stripped %q", got)
	}
}
