package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/session"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeOAuth struct {
	authURL     string
	authErr     error
	returnTo    string
	stateErr    error
	user        *oauth.GoogleUser
	exchangeErr error
}

func (f *fakeOAuth) AuthCodeURL(context.Context, string) (string, error) {
	if f.authErr != nil {
		return "", f.authErr
	}
	if f.authURL != "" {
		return f.authURL, nil
	}
	return "https://accounts.google.com/o/oauth2/auth?state=test", nil
}

func (f *fakeOAuth) ConsumeState(context.Context, string) (string, error) {
	if f.stateErr != nil {
		return "", f.stateErr
	}
	if f.returnTo != "" {
		return f.returnTo, nil
	}
	return "https://filenkov.store", nil
}

func (f *fakeOAuth) Exchange(context.Context, string) (*oauth.GoogleUser, error) {
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	if f.user != nil {
		return f.user, nil
	}
	return &oauth.GoogleUser{Sub: "sub-1", Email: "user@example.com", Name: "User"}, nil
}

type harness struct {
	t    *testing.T
	h    http.Handler
	mem  *users.Memory
	sess *session.Store
	oa   *fakeOAuth
	cfg  *config.Config
}

func newHarness(t *testing.T, oa *fakeOAuth) *harness {
	t.Helper()
	if oa == nil {
		oa = &fakeOAuth{}
	}
	cfg := &config.Config{
		CookieName:   "auth_session",
		CookieSecure: false,
		SessionTTL:   time.Hour,
		PublicOrigin: "https://filenkov.store",
		Whitelist:    config.DefaultWhitelist(),
		DevHTTP:      true,
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	mem := users.NewMemory("admin@filenkov.store")
	sess := session.NewStore(rdb, time.Hour)
	lim := ratelimit.New(rdb)
	h := New(cfg, mem, sess, oa, lim, func() error { return nil }, func() error { return nil }).Handler()
	return &harness{t: t, h: h, mem: mem, sess: sess, oa: oa, cfg: cfg}
}

func (hr *harness) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	hr.h.ServeHTTP(rec, req)
	return rec
}

func (hr *harness) login(u users.User) (*users.User, string) {
	hr.t.Helper()
	stored := hr.mem.Put(u)
	sid, err := hr.sess.Create(context.Background(), stored.ID)
	if err != nil {
		hr.t.Fatal(err)
	}
	return &stored, sid
}

func withCookie(req *http.Request, sid string) *http.Request {
	req.AddCookie(&http.Cookie{Name: "auth_session", Value: sid})
	return req
}

func TestHealth(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHealthUnhealthy(t *testing.T) {
	cfg := &config.Config{CookieName: "auth_session", SessionTTL: time.Hour, Whitelist: config.DefaultWhitelist()}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h := New(cfg, users.NewMemory(""), session.NewStore(rdb, time.Hour), &fakeOAuth{}, ratelimit.New(rdb),
		func() error { return io.ErrUnexpectedEOF },
		func() error { return nil },
	).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeUnauthorized(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeOK(t *testing.T) {
	hr := newHarness(t, nil)
	u, sid := hr.login(users.User{
		GoogleSub: "s", GoogleEmail: "a@b.c", Name: "Ann",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleFamily, ResumeRoles: users.ServiceRoleStranger,
	})
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/me", nil), sid)
	rec := hr.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body userJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != u.ID.String() || body.Name != "Ann" || body.StorageRoles != "FAMILY" {
		t.Fatalf("%+v", body)
	}
}

func TestMeBlocked(t *testing.T) {
	hr := newHarness(t, nil)
	_, sid := hr.login(users.User{
		GoogleSub: "s", GoogleEmail: "a@b.c", Name: "Ann",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
		Blocked: true,
	})
	rec := hr.do(withCookie(httptest.NewRequest(http.MethodGet, "/api/me", nil), sid))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	hr := newHarness(t, nil)
	_, sid := hr.login(users.User{
		GoogleSub: "s", GoogleEmail: "a@b.c", Name: "Ann",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	rec := hr.do(withCookie(httptest.NewRequest(http.MethodPost, "/api/logout", nil), sid))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if _, err := hr.sess.Get(context.Background(), sid); err != session.ErrNotFound {
		t.Fatalf("session still live: %v", err)
	}
}

func TestAdminForbiddenForStranger(t *testing.T) {
	hr := newHarness(t, nil)
	_, sid := hr.login(users.User{
		GoogleSub: "s", GoogleEmail: "a@b.c", Name: "Ann",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	rec := hr.do(withCookie(httptest.NewRequest(http.MethodGet, "/api/admin/users", nil), sid))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAdminListPatchBlock(t *testing.T) {
	hr := newHarness(t, nil)
	admin, sid := hr.login(users.User{
		GoogleSub: "admin", GoogleEmail: "admin@filenkov.store", Name: "Boss",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	target := hr.mem.Put(users.User{
		GoogleSub: "t", GoogleEmail: "t@x", Name: "Target",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})

	list := hr.do(withCookie(httptest.NewRequest(http.MethodGet, "/api/admin/users", nil), sid))
	if list.Code != http.StatusOK {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	var listed struct{ Users []userJSON `json:"users"` }
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Users) != 2 {
		t.Fatalf("want 2 users, got %d", len(listed.Users))
	}

	family := users.ServiceRoleFamily
	body, _ := json.Marshal(map[string]string{"storage_roles": string(family)})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+target.ID.String(), bytes.NewReader(body))
	rec := hr.do(withCookie(req, sid))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}

	block, _ := json.Marshal(map[string]bool{"blocked": true})
	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/"+target.ID.String()+"/block", bytes.NewReader(block))
	rec = hr.do(withCookie(req, sid))
	if rec.Code != http.StatusOK {
		t.Fatalf("block %d %s", rec.Code, rec.Body.String())
	}

	demote, _ := json.Marshal(map[string]string{"roles": "STRANGER"})
	req = httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+admin.ID.String(), bytes.NewReader(demote))
	rec = hr.do(withCookie(req, sid))
	if rec.Code != http.StatusConflict {
		t.Fatalf("last admin demote: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPatchInvalidJSONAndID(t *testing.T) {
	hr := newHarness(t, nil)
	_, sid := hr.login(users.User{
		GoogleSub: "admin", GoogleEmail: "a@x", Name: "A",
		Roles: users.RoleAdmin, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/not-a-uuid", bytes.NewReader([]byte(`{}`)))
	rec := hr.do(withCookie(req, sid))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("id %d", rec.Code)
	}

	id := uuid.New()
	req = httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+id.String(), bytes.NewReader([]byte(`{`)))
	rec = hr.do(withCookie(req, sid))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("json %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+id.String(), bytes.NewReader([]byte(`{"roles":"USER"}`)))
	rec = hr.do(withCookie(req, sid))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("role %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+id.String(), bytes.NewReader([]byte(`{"roles":"ADMIN"}`)))
	rec = hr.do(withCookie(req, sid))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing %d", rec.Code)
	}
}

func TestOAuthStartRejectsBadReturnTo(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://evil.example/", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestOAuthStartRedirect(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{authURL: "https://accounts.google.com/auth?state=xyz"})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://storage.filenkov.store/app", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://accounts.google.com/auth?state=xyz" {
		t.Fatalf("location %q", loc)
	}
}

func TestOAuthCallbackSuccess(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		returnTo: "https://storage.filenkov.store/",
		user:     &oauth.GoogleUser{Sub: "g1", Email: "new@x.com", Name: "New"},
	})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "https://storage.filenkov.store/" {
		t.Fatalf("location %q", rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth_session" && c.Value != "" && c.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing session cookie: %+v", cookies)
	}
}

func TestOAuthCallbackBlocked(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		user: &oauth.GoogleUser{Sub: "blocked", Email: "b@x.com", Name: "B"},
	})
	hr.mem.Put(users.User{
		GoogleSub: "blocked", GoogleEmail: "b@x.com", Name: "B",
		Roles: users.RoleStranger, StorageRoles: users.ServiceRoleStranger, ResumeRoles: users.ServiceRoleStranger,
		Blocked: true,
	})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthCallbackBadInput(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{stateErr: oauth.ErrInvalidState})
	if rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?error=access_denied", nil)); rec.Code != http.StatusBadRequest {
		t.Fatalf("provider error %d", rec.Code)
	}
	if rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback", nil)); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing %d", rec.Code)
	}
	if rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=x&state=y", nil)); rec.Code != http.StatusBadRequest {
		t.Fatalf("state %d", rec.Code)
	}
}

func TestOAuthRateLimit(t *testing.T) {
	hr := newHarness(t, nil)
	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		last = hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://filenkov.store/", nil))
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 11 hits, got %d", last.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("xff %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.4")
	if got := clientIP(req); got != "198.51.100.4" {
		t.Fatalf("xrip %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	if got := clientIP(req); got != "192.0.2.1" {
		t.Fatalf("remote %q", got)
	}
}

func TestCookieDomain(t *testing.T) {
	cfg := &config.Config{
		CookieName:   "auth_session",
		CookieDomain: ".filenkov.store",
		CookieSecure: true,
		SessionTTL:   time.Hour,
		Whitelist:    config.DefaultWhitelist(),
	}
	s := New(cfg, users.NewMemory(""), nil, &fakeOAuth{}, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "sid")
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %v", cookies)
	}
	c := cookies[0]
	// net/http strips the leading dot when serializing Domain.
	if c.Domain != "filenkov.store" && c.Domain != ".filenkov.store" {
		t.Fatalf("domain %q", c.Domain)
	}
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("%+v", c)
	}
}
