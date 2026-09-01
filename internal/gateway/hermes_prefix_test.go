package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHermesDroppedAssetRedirect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
		ref  string
		want string
	}{
		{"chat page lazy chunk", "/assets/ChatPage-abc.js", "https://filenkov.store/hermes/chat", "/hermes/assets/ChatPage-abc.js"},
		{"sessions page css", "/assets/xterm.css", "https://filenkov.store/hermes/sessions", "/hermes/assets/xterm.css"},
		{"hub referer stays hub", "/assets/index.js", "https://filenkov.store/", ""},
		{"no referer stays hub", "/assets/ChatPage-abc.js", "", ""},
		{"already prefixed", "/hermes/assets/ChatPage-abc.js", "https://filenkov.store/hermes/chat", ""},
		{"non-asset ignored", "/api/me", "https://filenkov.store/hermes/chat", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.ref != "" {
				r.Header.Set("Referer", tc.ref)
			}
			if got := hermesDroppedAssetRedirect(r); got != tc.want {
				t.Fatalf("redirect %q want %q", got, tc.want)
			}
		})
	}
}

func TestPrefixPublicPath(t *testing.T) {
	cases := []struct {
		path, prefix, want string
	}{
		{"/", "/hermes", "/hermes/"},
		{"", "/hermes", "/hermes/"},
		{"/chat", "/hermes", "/hermes/chat"},
		{"/hermes/", "/hermes", "/hermes/"},
		{"/hermes/chat", "/hermes", "/hermes/chat"},
		{"https://evil.example/", "/hermes", "https://evil.example/"},
		{"//evil.example/", "/hermes", "//evil.example/"},
		{"/", "", "/"},
		{"rel", "/hermes", "rel"},
	}
	for _, tc := range cases {
		if got := prefixPublicPath(tc.path, tc.prefix); got != tc.want {
			t.Errorf("prefixPublicPath(%q, %q)=%q want %q", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestHermesBrowserPath(t *testing.T) {
	cases := []struct {
		path, prefix, want string
	}{
		{"/", "/hermes", "/hermes/sessions"},
		{"", "/hermes", "/hermes/sessions"},
		{"/login", "/hermes", "/hermes/sessions"},
		{"/hermes/", "/hermes", "/hermes/sessions"},
		{"/hermes/login", "/hermes", "/hermes/sessions"},
		{"/sessions", "/hermes", "/hermes/sessions"},
		{"/hermes/sessions", "/hermes", "/hermes/sessions"},
		{"/chat", "/hermes", "/hermes/chat"},
	}
	for _, tc := range cases {
		if got := hermesBrowserPath(tc.path, tc.prefix); got != tc.want {
			t.Errorf("hermesBrowserPath(%q, %q)=%q want %q", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestPrefixPublicURLKeepsHermesInternalNext(t *testing.T) {
	got := prefixPublicURL("/login?next=/", "/hermes")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/hermes/login" {
		t.Fatalf("path %q", got)
	}
	if u.Query().Get("next") != "/" {
		t.Fatalf("next %q want / (leave Hermes-internal root)", got)
	}
	if got := prefixPublicURL("https://filenkov.store/", "/hermes"); got != "https://filenkov.store/" {
		t.Fatalf("absolute URL rewritten: %q", got)
	}
}

func TestRewriteHermesNextQueryStripsPublicPrefix(t *testing.T) {
	u, _ := url.Parse("/login?next=%2Fhermes%2Flogin")
	rewriteHermesNextQuery(u, "/hermes")
	if u.Query().Get("next") != "/sessions" {
		t.Fatalf("login next %q", u.RawQuery)
	}
	u, _ = url.Parse("/login?next=%2Fhermes%2Fsessions")
	rewriteHermesNextQuery(u, "/hermes")
	if u.Query().Get("next") != "/hermes/sessions" {
		t.Fatalf("sessions next must stay public, got %q", u.RawQuery)
	}
}

func TestRewriteHermesNextJSON(t *testing.T) {
	out, ok := rewriteHermesNextJSON([]byte(`{"ok":true,"next":"/"}`), "/hermes")
	if !ok {
		t.Fatal("expected rewrite")
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["next"] != "/hermes/sessions" || payload["ok"] != true {
		t.Fatalf("payload %v", payload)
	}
	if _, ok := rewriteHermesNextJSON([]byte(`{"ok":true,"next":"/hermes/sessions"}`), "/hermes"); ok {
		t.Fatal("already-prefixed sessions next must stay")
	}
	if _, ok := rewriteHermesNextJSON([]byte(`{"status":"ok"}`), "/hermes"); ok {
		t.Fatal("no next field")
	}
}
