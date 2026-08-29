package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/rejected"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
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

type dirUser struct {
	usersvc.User
	roles []usersvc.ServiceRole
}

type dirMock struct {
	mu     sync.Mutex
	bySub  map[string]*dirUser
	byID   map[string]*dirUser
	failID int
}

func newDir() *dirMock {
	return &dirMock{bySub: map[string]*dirUser{}, byID: map[string]*dirUser{}}
}

func (m *dirMock) put(u usersvc.User, roles ...usersvc.ServiceRole) {
	cp := &dirUser{User: u, roles: roles}
	m.bySub[u.GoogleSub] = cp
	m.byID[u.ID] = cp
}

func (m *dirMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeErr := func(status int, code, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": msg}})
	}
	if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		writeErr(http.StatusBadRequest, "VALIDATION_ERROR", "jwt must not be sent")
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/users":
		var body usersvc.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(http.StatusBadRequest, "INVALID_JSON", "invalid json")
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.bySub[body.GoogleSub]; ok {
			writeErr(http.StatusConflict, usersvc.CodeGoogleSubExists, "exists")
			return
		}
		for _, u := range m.byID {
			if strings.EqualFold(u.Username, body.Username) {
				writeErr(http.StatusConflict, usersvc.CodeUsernameExists, "exists")
				return
			}
		}
		role := usersvc.RoleStranger
		if len(m.byID) == 0 {
			role = usersvc.RoleAdmin
		}
		u := usersvc.User{
			ID: uuid.NewString(), GoogleSub: body.GoogleSub, Email: body.Email,
			Username: body.Username, GlobalRole: role,
		}
		m.put(u)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(u)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/by-google-sub/"):
		sub := strings.TrimPrefix(r.URL.Path, "/users/by-google-sub/")
		m.mu.Lock()
		u, ok := m.bySub[sub]
		m.mu.Unlock()
		if !ok {
			writeErr(http.StatusNotFound, usersvc.CodeNotFound, "User not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u.User)
		return
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/roles"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/users/"), "/roles")
		m.mu.Lock()
		u, ok := m.byID[id]
		m.mu.Unlock()
		if !ok {
			writeErr(http.StatusNotFound, usersvc.CodeNotFound, "User not found")
			return
		}
		svcs := u.roles
		if svcs == nil {
			svcs = []usersvc.ServiceRole{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(usersvc.RolesDocument{UserID: u.ID, GlobalRole: u.GlobalRole, Services: svcs})
		return
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/"):
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		if strings.Contains(id, "/") {
			writeErr(http.StatusNotFound, usersvc.CodeNotFound, "User not found")
			return
		}
		m.mu.Lock()
		fail := m.failID
		u, ok := m.byID[id]
		m.mu.Unlock()
		if fail != 0 {
			code := usersvc.CodeNotFound
			if fail == http.StatusForbidden {
				code = usersvc.CodeForbidden
			}
			writeErr(fail, code, "forced")
			return
		}
		if !ok {
			writeErr(http.StatusNotFound, usersvc.CodeNotFound, "User not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u.User)
		return
	}
	writeErr(http.StatusNotFound, usersvc.CodeNotFound, "User not found")
}

type harness struct {
	t        *testing.T
	h        http.Handler
	dir      *dirMock
	tok      *token.Service
	rej      *rejected.Cache
	oa       *fakeOAuth
	cfg      *config.Config
	users    *usersvc.Client
	sessions *sid.Store
}

func newHarness(t *testing.T, oa *fakeOAuth) *harness {
	t.Helper()
	if oa == nil {
		oa = &fakeOAuth{}
	}
	cfg := &config.Config{
		CookieSecure:  false,
		PublicOrigin:  "https://filenkov.store",
		DevHTTP:       true,
		JWTSecret:     "test-jwt-secret-key-32-bytes-ok!",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 168 * time.Hour,
	}
	cfg.SetGatewayHosts([]string{"filenkov.store"})
	tok, err := token.New(token.Config{
		Secret:     []byte(cfg.JWTSecret),
		Issuer:     cfg.PublicOrigin,
		Audiences:  []string{"filenkov.store", "localhost"},
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	dir := newDir()
	us := httptest.NewServer(dir)
	t.Cleanup(us.Close)
	client, err := usersvc.New(us.URL, us.Client())
	if err != nil {
		t.Fatal(err)
	}
	rej := rejected.New(rdb)
	sessions := sid.New(rdb)
	h := New(cfg, tok, rej, client, oa, ratelimit.New(rdb), func() error { return nil }, sessions).Handler()
	return &harness{t: t, h: h, dir: dir, tok: tok, rej: rej, oa: oa, cfg: cfg, users: client, sessions: sessions}
}

func (hr *harness) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	hr.h.ServeHTTP(rec, req)
	return rec
}

func (hr *harness) issue(u usersvc.User, roles ...usersvc.ServiceRole) (access, refresh string) {
	hr.t.Helper()
	hr.dir.put(u, roles...)
	a, _, err := hr.tok.IssueAccess(u.ID, u.Email, u.Username, string(u.GlobalRole))
	if err != nil {
		hr.t.Fatal(err)
	}
	r, _, err := hr.tok.IssueRefresh(u.ID)
	if err != nil {
		hr.t.Fatal(err)
	}
	return a, r
}

func withAccess(req *http.Request, access string) *http.Request {
	req.AddCookie(&http.Cookie{Name: token.CookieAccess, Value: access})
	return req
}

func withRefresh(req *http.Request, refresh string) *http.Request {
	req.AddCookie(&http.Cookie{Name: token.CookieRefresh, Value: refresh})
	return req
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestHealth(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestHealthUnhealthy(t *testing.T) {
	hr := newHarness(t, nil)
	h := New(hr.cfg, hr.tok, hr.rej, hr.users, &fakeOAuth{}, ratelimit.New(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})),
		func() error { return io.ErrUnexpectedEOF },
		nil,
	).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeUnauthorizedMissingCookie(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeOK(t *testing.T) {
	hr := newHarness(t, nil)
	id := uuid.NewString()
	access, _ := hr.issue(usersvc.User{
		ID: id, GoogleSub: "g1", Email: "ada@example.com", Username: "ada", GlobalRole: usersvc.RoleAdmin,
	}, usersvc.ServiceRole{Service: "storage", Role: usersvc.RoleFamily})
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body meJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != id || body.Username != "ada" || body.Email != "ada@example.com" || body.GlobalRole != "ADMIN" {
		t.Fatalf("%+v", body)
	}
	if body.GoogleSub != "g1" || len(body.Services) != 1 || body.Services[0].Service != "storage" {
		t.Fatalf("%+v", body)
	}
}

func TestMeBlocked(t *testing.T) {
	hr := newHarness(t, nil)
	access, _ := hr.issue(usersvc.User{
		ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "ann", GlobalRole: usersvc.RoleBlocked,
	})
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeUserService404(t *testing.T) {
	hr := newHarness(t, nil)
	id := uuid.NewString()
	access, _, err := hr.tok.IssueAccess(id, "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestMeUserService403(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger}
	access, _ := hr.issue(u)
	hr.dir.failID = http.StatusForbidden
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestMeTamperedJWT(t *testing.T) {
	hr := newHarness(t, nil)
	access, _ := hr.issue(usersvc.User{
		ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger,
	})
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access[:len(access)-2]+"xx"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeExpiredJWT(t *testing.T) {
	cfg := &config.Config{PublicOrigin: "https://filenkov.store", DevHTTP: true, JWTSecret: "test-jwt-secret-key-32-bytes-ok!"}
	tok, err := token.New(token.Config{
		Secret: []byte(cfg.JWTSecret), Issuer: cfg.PublicOrigin,
		Audiences: []string{"filenkov.store"}, AccessTTL: time.Hour, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	dir := newDir()
	us := httptest.NewServer(dir)
	t.Cleanup(us.Close)
	client, err := usersvc.New(us.URL, us.Client())
	if err != nil {
		t.Fatal(err)
	}
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger}
	dir.put(u)
	now := time.Now().Add(-time.Hour)
	claims := jwt.MapClaims{
		"sub": u.ID, "jti": uuid.NewString(), "iss": cfg.PublicOrigin,
		"aud": []string{"filenkov.store"}, "typ": token.TypAccess,
		"email": u.Email, "username": u.Username, "global_role": string(u.GlobalRole),
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	h := New(cfg, tok, rejected.New(rdb), client, &fakeOAuth{}, ratelimit.New(rdb), func() error { return nil }, nil).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), raw))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMeRejectedJTI(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger}
	access, _ := hr.issue(u)
	claims, err := hr.tok.ParseAccess(access)
	if err != nil {
		t.Fatal(err)
	}
	if err := hr.rej.RejectJTI(context.Background(), claims.JTI(), time.Hour); err != nil {
		t.Fatal(err)
	}
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLogoutRejectsJTI(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger}
	access, refresh := hr.issue(u)
	req := withRefresh(withAccess(httptest.NewRequest(http.MethodPost, "/api/logout", nil), access), refresh)
	rec := hr.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if c := cookieNamed(rec.Result().Cookies(), token.CookieAccess); c == nil || c.MaxAge >= 0 {
		t.Fatalf("access cookie not cleared: %+v", rec.Result().Cookies())
	}
	rec = hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reuse access %d", rec.Code)
	}
	rec = hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reuse refresh %d %s", rec.Code, rec.Body.String())
	}
}

func TestLogoutMissingCookiesOK(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodPost, "/api/logout", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRefreshOK(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "ada@example.com", Username: "ada", GlobalRole: usersvc.RoleFamily}
	_, refresh := hr.issue(u)
	rec := hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	newAccess := cookieNamed(rec.Result().Cookies(), token.CookieAccess)
	newRefresh := cookieNamed(rec.Result().Cookies(), token.CookieRefresh)
	newSession := cookieNamed(rec.Result().Cookies(), sid.CookieName)
	if newAccess == nil || newAccess.Value == "" || newRefresh == nil || newRefresh.Value == "" {
		t.Fatalf("cookies %+v", rec.Result().Cookies())
	}
	if newSession == nil || newSession.Value == "" {
		t.Fatal("refresh without existing sid must mint auth_session")
	}
	if newRefresh.Value == refresh {
		t.Fatal("refresh must rotate")
	}
	rec = hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), newAccess.Value))
	if rec.Code != http.StatusOK {
		t.Fatalf("me after refresh %d", rec.Code)
	}
	rec = hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh %d", rec.Code)
	}
}

func TestRefreshInvalid(t *testing.T) {
	hr := newHarness(t, nil)
	rec := hr.do(httptest.NewRequest(http.MethodPost, "/api/refresh", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing %d", rec.Code)
	}
	access, _ := hr.issue(usersvc.User{
		ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleStranger,
	})
	rec = hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), access))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("access as refresh %d", rec.Code)
	}
}

func TestRefreshBlocked(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleBlocked}
	_, refresh := hr.issue(u)
	rec := hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshUserGone(t *testing.T) {
	hr := newHarness(t, nil)
	id := uuid.NewString()
	refresh, _, err := hr.tok.IssueRefresh(id)
	if err != nil {
		t.Fatal(err)
	}
	rec := hr.do(withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAdminRoutesGone(t *testing.T) {
	hr := newHarness(t, nil)
	u := usersvc.User{ID: uuid.NewString(), GoogleSub: "g", Email: "a@b.c", Username: "a", GlobalRole: usersvc.RoleAdmin}
	access, _ := hr.issue(u)
	rec := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/admin/users", nil), access))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("list %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+u.ID, strings.NewReader(`{"roles":"FAMILY"}`))
	rec = hr.do(withAccess(req, access))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch %d", rec.Code)
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
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://filenkov.store/app", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://accounts.google.com/auth?state=xyz" {
		t.Fatalf("location %q", loc)
	}
}

func TestOAuthCallbackCreatesUser(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		returnTo: "https://filenkov.store/",
		user:     &oauth.GoogleUser{Sub: "110000000000000000001", Email: "ada@example.com", Name: "Ada"},
	})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "https://filenkov.store/" {
		t.Fatalf("location %q", rec.Header().Get("Location"))
	}
	access := cookieNamed(rec.Result().Cookies(), token.CookieAccess)
	refresh := cookieNamed(rec.Result().Cookies(), token.CookieRefresh)
	if access == nil || access.Value == "" || !access.HttpOnly || refresh == nil || refresh.Value == "" {
		t.Fatalf("cookies %+v", rec.Result().Cookies())
	}
	session := cookieNamed(rec.Result().Cookies(), sid.CookieName)
	if session == nil || session.Value == "" || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("auth_session missing: %+v", rec.Result().Cookies())
	}
	uid, err := hr.sessions.Get(context.Background(), session.Value)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := hr.tok.ParseAccess(access.Value)
	if err != nil {
		t.Fatal(err)
	}
	if uid != claims.Subject {
		t.Fatalf("session user %q jwt sub %q", uid, claims.Subject)
	}
	if claims.Username != "ada" || claims.Subject == "110000000000000000001" {
		t.Fatalf("%+v", claims)
	}
	me := hr.do(withAccess(httptest.NewRequest(http.MethodGet, "/api/me", nil), access.Value))
	if me.Code != http.StatusOK {
		t.Fatalf("me %d %s", me.Code, me.Body.String())
	}
}

func TestOAuthCallbackExistingNoCreate(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		user: &oauth.GoogleUser{Sub: "g1", Email: "ada@example.com", Name: "Ada"},
	})
	id := uuid.NewString()
	hr.dir.put(usersvc.User{ID: id, GoogleSub: "g1", Email: "old@x.com", Username: "oldhandle", GlobalRole: usersvc.RoleStranger})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	access := cookieNamed(rec.Result().Cookies(), token.CookieAccess)
	claims, err := hr.tok.ParseAccess(access.Value)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != id || claims.Username != "oldhandle" {
		t.Fatalf("must reuse directory row: %+v", claims)
	}
}

func TestOAuthCallbackBlocked(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		user: &oauth.GoogleUser{Sub: "blocked", Email: "b@x.com", Name: "B"},
	})
	hr.dir.put(usersvc.User{
		ID: uuid.NewString(), GoogleSub: "blocked", Email: "b@x.com", Username: "b", GlobalRole: usersvc.RoleBlocked,
	})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if cookieNamed(rec.Result().Cookies(), token.CookieAccess) != nil {
		t.Fatal("must not issue cookies for BLOCKED")
	}
	if cookieNamed(rec.Result().Cookies(), sid.CookieName) != nil {
		t.Fatal("must not issue auth_session for BLOCKED")
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
		CookieDomain:  ".filenkov.store",
		CookieSecure:  true,
		PublicOrigin:  "https://filenkov.store",
		JWTSecret:     "test-jwt-secret-key-32-bytes-ok!",
		JWTAccessTTL:  time.Hour,
		JWTRefreshTTL: time.Hour,
	}
	tok, err := token.New(token.Config{
		Secret: []byte(cfg.JWTSecret), Issuer: cfg.PublicOrigin,
		Audiences: []string{"filenkov.store"}, AccessTTL: time.Hour, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, tok, nil, nil, &fakeOAuth{}, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.setAuthCookies(rec, "acc", "ref")
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies %v", cookies)
	}
	for _, c := range cookies {
		if c.Domain != "filenkov.store" && c.Domain != ".filenkov.store" {
			t.Fatalf("domain %q", c.Domain)
		}
		if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
			t.Fatalf("%+v", c)
		}
	}
}

func TestOAuthStartUsesGatewayHosts(t *testing.T) {
	hr := newHarness(t, nil)
	hr.cfg.SetGatewayHosts([]string{"filenkov.store"})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://storage.filenkov.store/app", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("storage host must be rejected when not in gateway YAML, status %d", rec.Code)
	}
	rec = hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://filenkov.store/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("hub host: status %d %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthStartAllowsStorageHost(t *testing.T) {
	hr := newHarness(t, nil)
	hr.cfg.SetGatewayHosts([]string{"filenkov.store", "storage.filenkov.store"})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google?return_to=https://storage.filenkov.store/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("storage host in YAML must be allowed, status %d %s", rec.Code, rec.Body.String())
	}
}

func TestAuthSessionStableAcrossRefresh(t *testing.T) {
	hr := newHarness(t, &fakeOAuth{
		returnTo: "https://filenkov.store/",
		user:     &oauth.GoogleUser{Sub: "g-sid", Email: "sid@example.com", Name: "Sid"},
	})
	rec := hr.do(httptest.NewRequest(http.MethodGet, "/oauth/google/callback?code=abc&state=st", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	session := cookieNamed(rec.Result().Cookies(), sid.CookieName)
	refresh := cookieNamed(rec.Result().Cookies(), token.CookieRefresh)
	if session == nil || refresh == nil {
		t.Fatalf("cookies %+v", rec.Result().Cookies())
	}
	first := session.Value

	req := withRefresh(httptest.NewRequest(http.MethodPost, "/api/refresh", nil), refresh.Value)
	req.AddCookie(&http.Cookie{Name: sid.CookieName, Value: first})
	rec = hr.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh %d %s", rec.Code, rec.Body.String())
	}
	kept := cookieNamed(rec.Result().Cookies(), sid.CookieName)
	if kept == nil || kept.Value != first {
		t.Fatalf("sid must be stable across refresh, first=%q kept=%v", first, kept)
	}
	uid, err := hr.sessions.Get(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if uid == "" {
		t.Fatal("redis mapping missing after refresh")
	}

	logout := withRefresh(withAccess(httptest.NewRequest(http.MethodPost, "/api/logout", nil), cookieNamed(rec.Result().Cookies(), token.CookieAccess).Value), cookieNamed(rec.Result().Cookies(), token.CookieRefresh).Value)
	logout.AddCookie(&http.Cookie{Name: sid.CookieName, Value: first})
	rec = hr.do(logout)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout %d", rec.Code)
	}
	cleared := cookieNamed(rec.Result().Cookies(), sid.CookieName)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("auth_session not cleared: %+v", rec.Result().Cookies())
	}
	if _, err := hr.sessions.Get(context.Background(), first); err == nil {
		t.Fatal("redis session key must be deleted on logout")
	}
}
