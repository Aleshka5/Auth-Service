package gateway

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
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

type probe struct {
	method          string
	path            string
	host            string
	rawQuery        string
	authID          string
	authEmail       string
	authName        string
	authRole        string
	storageRole     string
	userID          string
	cookie          string
	authorization   string
	serviceToken    string
	spoofedLeft     string
	forwardedHost   string
	forwardedPrefix string
	mu              sync.Mutex
	responder       func(http.ResponseWriter, *http.Request)
}

func (p *probe) setResponder(fn func(http.ResponseWriter, *http.Request)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.responder = fn
}

func newUpstream(t *testing.T) (*httptest.Server, *probe) {
	t.Helper()
	p := &probe{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.method = r.Method
		p.path = r.URL.Path
		p.host = r.Host
		p.rawQuery = r.URL.RawQuery
		p.authID = r.Header.Get("X-Auth-User-Id")
		p.authEmail = r.Header.Get("X-Auth-Email")
		p.authName = r.Header.Get("X-Auth-Name")
		p.authRole = r.Header.Get("X-Auth-Role")
		p.storageRole = r.Header.Get("X-Storage-Role")
		p.userID = r.Header.Get("X-User-Id")
		p.cookie = r.Header.Get("Cookie")
		p.authorization = r.Header.Get("Authorization")
		p.serviceToken = r.Header.Get("X-Service-Token")
		p.spoofedLeft = r.Header.Get("X-Auth-Spoof")
		p.forwardedHost = r.Header.Get("X-Forwarded-Host")
		p.forwardedPrefix = r.Header.Get("X-Forwarded-Prefix")
		fn := p.responder
		p.mu.Unlock()
		if fn != nil {
			fn(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, p
}

func yamlFor(product, userSvc string) []byte {
	return []byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
upstreams:
  api: "` + product + `"
  web: "` + product + `"
  hermes: "` + product + `"
  user_service: "` + userSvc + `"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /oauth/*
        upstream: api
        auth: public
      - match: /api/refresh
        upstream: api
        auth: public
      - match: /api/logout
        upstream: api
        auth: public
      - match: /api/me
        upstream: api
        auth: session
      - match: /api/*
        upstream: api
        auth: session
      - match: /users/by-google-sub/*
        auth: reject
      - match: /users/by-email/*
        auth: reject
      - match: /users
        methods: [POST]
        auth: reject
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin
        preserve_host: false
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
      - match: /health
        upstream: api
        auth: public
      - match: /hermes/login
        methods: [GET]
        auth: session
        redirect: /hermes/sessions
      - match: /hermes/*
        upstream: hermes
        auth: session
        strip_prefix: /hermes
      - match: /chat
        methods: [GET]
        auth: session
        redirect: /hermes/chat
      - match: /sessions
        methods: [GET]
        auth: session
        redirect: /hermes/sessions
      - match: /admin
        upstream: web
        auth: admin
      - match: /admin/*
        upstream: web
        auth: admin
      - match: /*
        upstream: web
        auth: public
`)
}

type gwHarness struct {
	t        *testing.T
	h        http.Handler
	tok      *token.Service
	rej      *rejected.Cache
	sessions *sid.Store
	up       *probe
	users    *probe
	secret   []byte
}

func testToken(t *testing.T, secret []byte, accessTTL time.Duration) *token.Service {
	t.Helper()
	s, err := token.New(token.Config{
		Secret:     secret,
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"filenkov.store", "localhost"},
		AccessTTL:  accessTTL,
		RefreshTTL: 168 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newGW(t *testing.T) *gwHarness {
	t.Helper()
	product, productProbe := newUpstream(t)
	userSvc, userProbe := newUpstream(t)
	file, err := Parse(yamlFor(product.URL, userSvc.URL))
	if err != nil {
		t.Fatal(err)
	}
	return newGWFromFile(t, file, productProbe, userProbe)
}

func newGWFromFile(t *testing.T, file *File, productProbe, userProbe *probe) *gwHarness {
	t.Helper()
	cfg := &config.Config{
		CookieSecure: false,
		PublicOrigin: "https://filenkov.store",
		DevHTTP:      true,
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := []byte("test-secret-key-32-bytes-long!!!")
	tok := testToken(t, secret, 15*time.Minute)
	rej := rejected.New(rdb)
	sessions := sid.New(rdb)
	s, err := New(cfg, file, tok, rej, ratelimit.New(rdb), nil, sessions)
	if err != nil {
		t.Fatal(err)
	}
	return &gwHarness{t: t, h: s.Handler(), tok: tok, rej: rej, sessions: sessions, up: productProbe, users: userProbe, secret: secret}
}

func (hr *gwHarness) do(req *http.Request) *httptest.ResponseRecorder {
	if req.Host == "" {
		req.Host = "filenkov.store"
	}
	rec := httptest.NewRecorder()
	hr.h.ServeHTTP(rec, req)
	return rec
}

func (hr *gwHarness) login(email, username, role string) (sub, raw string) {
	hr.t.Helper()
	sub = uuid.NewString()
	raw, _, err := hr.tok.IssueAccess(sub, email, username, role)
	if err != nil {
		hr.t.Fatal(err)
	}
	return sub, raw
}

func req(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = "filenkov.store"
	return r
}

func withAccess(r *http.Request, raw string) *http.Request {
	r.AddCookie(&http.Cookie{Name: token.CookieAccess, Value: raw})
	return r
}

func TestHealthzNotProxied(t *testing.T) {
	hr := newGW(t)
	rec := hr.do(req(http.MethodGet, "/healthz"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "" {
		t.Fatalf("healthz must not hit upstream, path=%q", hr.up.path)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestAnonymousPublicOK(t *testing.T) {
	hr := newGW(t)
	rec := hr.do(req(http.MethodGet, "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "/" {
		t.Fatalf("upstream path %q", hr.up.path)
	}
	if hr.up.authEmail != "" || hr.up.authID != "" {
		t.Fatalf("anonymous must not inject identity: %+v", hr.up)
	}
}

func TestPublicOAuthAndRefreshWithoutJWT(t *testing.T) {
	hr := newGW(t)
	rec := hr.do(req(http.MethodGet, "/oauth/google"))
	if rec.Code != http.StatusOK {
		t.Fatalf("oauth status %d", rec.Code)
	}
	rec = hr.do(req(http.MethodPost, "/api/refresh"))
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status %d", rec.Code)
	}
	rec = hr.do(req(http.MethodPost, "/api/logout"))
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status %d", rec.Code)
	}
}

func TestAnonymousSessionRedirectVs401(t *testing.T) {
	hr := newGW(t)

	rec := hr.do(req(http.MethodGet, "/admin"))
	if rec.Code != http.StatusFound {
		t.Fatalf("html admin status %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://filenkov.store/auth?return_to=") {
		t.Fatalf("location %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	rt := u.Query().Get("return_to")
	if rt != "https://filenkov.store/admin" {
		t.Fatalf("return_to %q", rt)
	}

	api := req(http.MethodGet, "/api/me")
	rec = hr.do(api)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/api/me status %d", rec.Code)
	}

	xhr := req(http.MethodGet, "/admin")
	xhr.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec = hr.do(xhr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("xhr status %d", rec.Code)
	}

	js := req(http.MethodGet, "/hermes/x")
	js.Header.Set("Accept", "application/json")
	rec = hr.do(js)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("json accept status %d", rec.Code)
	}

	users := req(http.MethodGet, "/users/"+uuid.NewString())
	rec = hr.do(users)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/users 401 status %d", rec.Code)
	}
}

func TestMissingInvalidExpiredRejectedJWT(t *testing.T) {
	hr := newGW(t)

	rec := hr.do(req(http.MethodGet, "/api/me"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing: %d", rec.Code)
	}

	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), "not-a-jwt"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid: %d", rec.Code)
	}

	expiredAt := time.Now().Add(-time.Hour)
	expiredClaims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    "https://filenkov.store",
			Audience:  jwt.ClaimStrings{"filenkov.store"},
			IssuedAt:  jwt.NewNumericDate(expiredAt),
			NotBefore: jwt.NewNumericDate(expiredAt),
			ExpiresAt: jwt.NewNumericDate(expiredAt.Add(time.Minute)),
		},
		Typ:        token.TypAccess,
		Email:      "e@x.c",
		Username:   "exp",
		GlobalRole: "STRANGER",
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(hr.secret)
	if err != nil {
		t.Fatal(err)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), expired))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired: %d %s", rec.Code, rec.Body.String())
	}

	other, err := token.New(token.Config{
		Secret:     []byte("other-secret-key-32-bytes-long!!!"),
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"filenkov.store"},
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	tampered, _, err := other.IssueAccess(uuid.NewString(), "e@x.c", "x", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), tampered))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered: %d", rec.Code)
	}

	_, raw := hr.login("ann@b.c", "ann", "STRANGER")
	parsed, err := hr.tok.ParseAccess(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := hr.rej.RejectJTI(context.Background(), parsed.JTI(), time.Hour); err != nil {
		t.Fatal(err)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), raw))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("rejected jti want 401, got %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "" {
		t.Fatalf("rejected jti must not proxy, path=%q", hr.up.path)
	}

	sub, raw := hr.login("bob@b.c", "bob", "STRANGER")
	if err := hr.rej.RejectUser(context.Background(), sub, time.Hour); err != nil {
		t.Fatal(err)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("rejected user want 403, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestOpenRedirectUnknownHost(t *testing.T) {
	hr := newGW(t)
	r := req(http.MethodGet, "/admin")
	r.Host = "evil.example"
	rec := hr.do(r)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect, got %q", loc)
	}
}

func TestStrangerForbiddenOnAdminRoutes(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("ann@b.c", "ann", "STRANGER")

	rec := hr.do(withAccess(req(http.MethodGet, "/hermes/"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("hermes status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "/" {
		t.Fatalf("hermes must proxy, upstream path %q", hr.up.path)
	}

	hr.up.path = ""
	rec = hr.do(withAccess(req(http.MethodGet, "/admin"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin status %d", rec.Code)
	}
	if hr.up.path != "" {
		t.Fatalf("must not proxy admin, upstream path %q", hr.up.path)
	}

	rec = hr.do(withAccess(req(http.MethodGet, "/api/me"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("session /api/me %d", rec.Code)
	}
}

func TestValidJWTInjectsIdentity(t *testing.T) {
	hr := newGW(t)
	sub, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/hermes/panel"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "/panel" {
		t.Fatalf("strip_prefix: upstream saw %q", hr.up.path)
	}
	if hr.up.host != "filenkov.store" {
		t.Fatalf("preserve Host: got %q", hr.up.host)
	}
	if hr.up.authEmail != "boss@b.c" || hr.up.authName != "boss" || hr.up.authRole != "ADMIN" {
		t.Fatalf("identity %+v", hr.up)
	}
	if hr.up.authID != sub {
		t.Fatalf("user id %q want %q", hr.up.authID, sub)
	}
	if hr.up.authorization != "" {
		t.Fatalf("hermes Authorization %q (must not inject hub JWT; it blocks cookie login)", hr.up.authorization)
	}
	if hr.up.forwardedPrefix != "/hermes" {
		t.Fatalf("X-Forwarded-Prefix %q", hr.up.forwardedPrefix)
	}
}

func TestHermesJSONRewritesNextToPublicPrefix(t *testing.T) {
	hr := newGW(t)
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"next":"/"}`)
	})
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	r := withAccess(req(http.MethodPost, "/hermes/api/auth/ws-ticket?next=%2F"), raw)
	r.Header.Set("Content-Type", "application/json")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["next"] != "/hermes/sessions" {
		t.Fatalf("next %v want /hermes/sessions", payload["next"])
	}
	q, err := url.ParseQuery(hr.up.rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("next") != "/" {
		t.Fatalf("upstream next query %q want / (do not rewrite /hermes/sessions down)", hr.up.rawQuery)
	}
}

func TestHermesLocationRewritesRootToPublicPrefix(t *testing.T) {
	hr := newGW(t)
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login?next=/", http.StatusFound)
	})
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/hermes/"), raw))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/hermes/login" {
		t.Fatalf("Location path %q", loc)
	}
	if u.Query().Get("next") != "/" {
		t.Fatalf("Location next %q want /", loc)
	}
}

func TestHermesOverwritesClientAuthorization(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	r := withAccess(req(http.MethodGet, "/hermes/chat"), raw)
	r.Header.Set("Authorization", "Bearer spoofed")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.authorization != "" {
		t.Fatalf("client Authorization survived: %q", hr.up.authorization)
	}
}

func TestHermesLoginSkippedWhenGatewayAuthed(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/hermes/login"), raw))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/hermes/sessions" {
		t.Fatalf("Location %q", loc)
	}
	if hr.up.path != "" {
		t.Fatalf("must not proxy Hermes login, upstream %q", hr.up.path)
	}
}

func TestHermesChatRedirectsToPublicPrefix(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/chat"), raw))
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/hermes/chat" {
		t.Fatalf("Location %q", loc)
	}
	if hr.up.path != "" {
		t.Fatalf("redirect must not proxy, upstream %q", hr.up.path)
	}
}

func TestHermesLazyAssetsRedirectWhenRefererIsHermes(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	r := withAccess(req(http.MethodGet, "/assets/ChatPage-abc.js"), raw)
	r.Header.Set("Referer", "https://filenkov.store/hermes/chat")
	rec := hr.do(r)
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/hermes/assets/ChatPage-abc.js" {
		t.Fatalf("Location %q", loc)
	}
	if hr.up.path != "" {
		t.Fatalf("redirect must not proxy, upstream %q", hr.up.path)
	}
}

func TestHubAssetsStayOnHubWithoutHermesReferer(t *testing.T) {
	hr := newGW(t)
	r := req(http.MethodGet, "/assets/index.js")
	r.Header.Set("Referer", "https://filenkov.store/")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.path != "/assets/index.js" {
		t.Fatalf("upstream path %q", hr.up.path)
	}
	if hr.up.forwardedPrefix != "" {
		t.Fatalf("hub must not get X-Forwarded-Prefix, got %q", hr.up.forwardedPrefix)
	}
}

func TestHermesInjectAccessJWTWhenEnabled(t *testing.T) {
	product, productProbe := newUpstream(t)
	userSvc, userProbe := newUpstream(t)
	yml := strings.Replace(
		string(yamlFor(product.URL, userSvc.URL)),
		"- match: /hermes/*\n        upstream: hermes\n        auth: session\n        strip_prefix: /hermes",
		"- match: /hermes/*\n        upstream: hermes\n        auth: session\n        strip_prefix: /hermes\n        inject_access_jwt: true",
		1,
	)
	file, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	hr := newGWFromFile(t, file, productProbe, userProbe)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/hermes/sessions"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.authorization != "Bearer "+raw {
		t.Fatalf("inject_access_jwt Authorization %q", hr.up.authorization)
	}
}

const wsClientKey = "dGhlIHNhbXBsZSBub25jZQ=="

func wsAcceptKey(clientKey string) string {
	sum := sha1.Sum([]byte(clientKey + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestHermesWebSocketUpgrade(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pty" {
			http.Error(w, "path "+r.URL.Path, http.StatusNotFound)
			return
		}
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "missing upgrade", http.StatusBadRequest)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijack", http.StatusInternalServerError)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(bufrw, "HTTP/1.1 101 Switching Protocols\r\n")
		_, _ = io.WriteString(bufrw, "Upgrade: websocket\r\n")
		_, _ = io.WriteString(bufrw, "Connection: Upgrade\r\n")
		_, _ = io.WriteString(bufrw, "Sec-WebSocket-Accept: "+wsAcceptKey(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
		_ = bufrw.Flush()
		_, _ = bufrw.Write([]byte("pty-ok"))
		_ = bufrw.Flush()
		time.Sleep(50 * time.Millisecond)
	}))
	t.Cleanup(up.Close)

	userSvc, userProbe := newUpstream(t)
	file, err := Parse(yamlFor(up.URL, userSvc.URL))
	if err != nil {
		t.Fatal(err)
	}
	hr := newGWFromFile(t, file, &probe{}, userProbe)
	edge := httptest.NewServer(hr.h)
	t.Cleanup(edge.Close)

	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	conn, err := net.Dial("tcp", edge.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	req := "GET /hermes/api/pty?ticket=t HTTP/1.1\r\n" +
		"Host: filenkov.store\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + wsClientKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Cookie: " + token.CookieAccess + "=" + raw + "\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status %d want 101", resp.StatusCode)
	}
	if got := resp.Header.Get("Upgrade"); !strings.EqualFold(got, "websocket") {
		t.Fatalf("Upgrade %q", got)
	}
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != wsAcceptKey(wsClientKey) {
		t.Fatalf("accept %q", got)
	}
	if resp.Header.Get("X-Frame-Options") != "" {
		t.Fatalf("security headers must not be applied to 101, got X-Frame-Options")
	}
	buf := make([]byte, 6)
	if _, err := io.ReadFull(br, buf); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if string(buf) != "pty-ok" {
		t.Fatalf("payload %q", buf)
	}
}

func TestHermesWebSocketAnonymousIs401(t *testing.T) {
	hr := newGW(t)
	edge := httptest.NewServer(hr.h)
	t.Cleanup(edge.Close)
	conn, err := net.Dial("tcp", edge.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	req := "GET /hermes/api/pty HTTP/1.1\r\n" +
		"Host: filenkov.store\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + wsClientKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401 (not a login redirect)", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		t.Fatalf("Location %q — WS must not 302 to /auth", loc)
	}
}

func TestAPIUpstreamDoesNotGetHermesBearer(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	rec := hr.do(withAccess(req(http.MethodGet, "/api/me"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.up.authorization != "" {
		t.Fatalf("api Authorization should stay empty, got %q", hr.up.authorization)
	}
	if hr.up.forwardedPrefix != "" {
		t.Fatalf("api should not get X-Forwarded-Prefix, got %q", hr.up.forwardedPrefix)
	}
}

func TestBlockedForbiddenOnSessionRoutes(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("b@b.c", "blocked", "BLOCKED")
	rec := hr.do(withAccess(req(http.MethodGet, "/api/me"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/admin"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin status %d", rec.Code)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("public home should still work for BLOCKED, %d", rec.Code)
	}
}

func TestStripSpoofedAuthHeaders(t *testing.T) {
	hr := newGW(t)
	sub, raw := hr.login("ann@b.c", "ann", "STRANGER")
	r := withAccess(req(http.MethodGet, "/api/me"), raw)
	r.Header.Set("X-Auth-User-Id", "spoof-id")
	r.Header.Set("X-Auth-Email", "evil@example.com")
	r.Header.Set("X-Auth-Role", "ADMIN")
	r.Header.Set("X-Auth-Spoof", "yes")
	r.Header.Set("X-User-Id", "spoof-user")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if hr.up.authEmail != "ann@b.c" || hr.up.authRole != "STRANGER" || hr.up.authName != "ann" {
		t.Fatalf("injected %+v", hr.up)
	}
	if hr.up.authID != sub || hr.up.authID == "spoof-id" {
		t.Fatalf("spoofed id survived: %+v", hr.up)
	}
	if hr.up.spoofedLeft != "" {
		t.Fatal("X-Auth-Spoof should be stripped")
	}
	if hr.up.userID != "" {
		t.Fatalf("product upstream must not get X-User-Id, got %q", hr.up.userID)
	}
}

func TestUserServiceProxySetsXUserIdStripsJWT(t *testing.T) {
	hr := newGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "ADMIN")
	id := uuid.NewString()
	r := withAccess(req(http.MethodGet, "/users/"+id), raw)
	r.Header.Set("Authorization", "Bearer "+raw)
	r.Header.Set("X-User-Id", "client-spoof")
	q := r.URL.Query()
	q.Set("access_token", raw)
	q.Set("keep", "1")
	r.URL.RawQuery = q.Encode()
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.users.path != "/users/"+id {
		t.Fatalf("path %q", hr.users.path)
	}
	if hr.users.userID != sub {
		t.Fatalf("X-User-Id %q want %q", hr.users.userID, sub)
	}
	if hr.users.cookie != "" {
		t.Fatalf("cookie forwarded: %q", hr.users.cookie)
	}
	if hr.users.authorization != "" {
		t.Fatalf("Authorization forwarded: %q", hr.users.authorization)
	}
	if hr.users.authID != "" || hr.users.authRole != "" {
		t.Fatalf("X-Auth-* must not go to User-Service: %+v", hr.users)
	}
	vals, _ := url.ParseQuery(hr.users.rawQuery)
	if vals.Get("access_token") != "" {
		t.Fatalf("jwt query forwarded: %q", hr.users.rawQuery)
	}
	if vals.Get("keep") != "1" {
		t.Fatalf("non-jwt query dropped: %q", hr.users.rawQuery)
	}
	if hr.up.path != "" {
		t.Fatalf("must not hit product upstream, path=%q", hr.up.path)
	}
}

func TestUserServiceCreateAndLookupRejected(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("ada@example.com", "ada", "ADMIN")

	rec := hr.do(withAccess(req(http.MethodPost, "/users"), raw))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /users %d", rec.Code)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/users/by-google-sub/11001"), raw))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("by-google-sub %d", rec.Code)
	}
	rec = hr.do(withAccess(req(http.MethodGet, "/users/by-email/ada@example.com"), raw))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("by-email %d", rec.Code)
	}
	if hr.users.path != "" {
		t.Fatalf("rejected paths must not proxy, path=%q", hr.users.path)
	}
}

func TestUserServicePutRejectsTargetUser(t *testing.T) {
	hr := newGW(t)
	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	target := uuid.NewString()
	sidVal, err := hr.sessions.Create(context.Background(), target, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	rec := hr.do(withAccess(req(http.MethodPut, "/users/"+target+"/role"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	ok, err := hr.rej.IsRejected(context.Background(), "", target, time.Now().Add(-time.Second))
	if err != nil || !ok {
		t.Fatalf("target user must be rejected after PUT: ok=%v err=%v", ok, err)
	}
	if _, err := hr.sessions.Get(context.Background(), sidVal); err == nil {
		t.Fatal("companion session must be deleted on rejected:user:")
	}
}

func TestUnknownHost404(t *testing.T) {
	hr := newGW(t)
	r := req(http.MethodGet, "/")
	r.Host = "llm.filenkov.store"
	rec := hr.do(r)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	hr := newGW(t)
	rec := hr.do(req(http.MethodGet, "/healthz"))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestPreserveHostDefault(t *testing.T) {
	hr := newGW(t)
	rec := hr.do(req(http.MethodGet, "/health"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if hr.up.host != "filenkov.store" {
		t.Fatalf("Host %q", hr.up.host)
	}
	if hr.up.path != "/health" {
		t.Fatalf("path %q", hr.up.path)
	}
}

func TestTargetUserID(t *testing.T) {
	id := "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	if got := targetUserID("/users/" + id + "/role"); got != id {
		t.Fatalf("role %q", got)
	}
	if got := targetUserID("/users/" + id + "/roles/storage"); got != id {
		t.Fatalf("service %q", got)
	}
	if got := targetUserID("/users/" + id); got != "" {
		t.Fatalf("get %q", got)
	}
	if got := targetUserID("/users/not-a-uuid/role"); got != "" {
		t.Fatalf("bad uuid %q", got)
	}
}

func yamlWithStorage(product, userSvc, storage string) []byte {
	return []byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
timeouts:
  proxy: 60s
upstreams:
  api: "` + product + `"
  web: "` + product + `"
  hermes: "` + product + `"
  user_service: "` + userSvc + `"
  storage: "` + storage + `"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /users
        methods: [POST]
        auth: reject
      - match: /users
        methods: [GET]
        upstream: user_service
        auth: admin
        preserve_host: false
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
      - match: /*
        upstream: web
        auth: public
  storage.filenkov.store:
    role_field: storage
    timeouts:
      proxy: 30m
    routes:
      - match: /health
        upstream: storage
        auth: public
      - match: /*
        upstream: storage
        auth: session
`)
}

// storageRoleMock serves GET /users/{id}/roles/{service} for one service
// (storage unless built with newServiceRoleMock).
type storageRoleMock struct {
	mu       sync.Mutex
	service  string
	role     map[string]string
	fail     map[string]int
	calls    int
	lastID   string
	lastPath string
	cookie   string
	authz    string
}

func newStorageRoleMock() *storageRoleMock {
	return newServiceRoleMock("storage")
}

func newServiceRoleMock(service string) *storageRoleMock {
	return &storageRoleMock{service: service, role: map[string]string{}, fail: map[string]int{}}
}

func (m *storageRoleMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.calls++
	m.lastPath = r.URL.Path
	m.cookie = r.Header.Get("Cookie")
	m.authz = r.Header.Get("Authorization")
	m.mu.Unlock()

	const prefix = "/users/"
	suffix := "/roles/" + m.service
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "NOT_FOUND", "message": "nope"}})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	m.mu.Lock()
	m.lastID = id
	fail := m.fail[id]
	role := m.role[id]
	m.mu.Unlock()
	if fail != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fail)
		code := "INTERNAL_ERROR"
		if fail == http.StatusNotFound {
			code = "NOT_FOUND"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": "forced"}})
		return
	}
	if role == "" {
		role = "STRANGER"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": id, "service": m.service, "role": role})
}

type storageHarness struct {
	*gwHarness
	storage *probe
	roles   *storageRoleMock
}

func newStorageGW(t *testing.T) *storageHarness {
	t.Helper()
	product, productProbe := newUpstream(t)
	storage, storageProbe := newUpstream(t)
	roles := newStorageRoleMock()
	roleSrv := httptest.NewServer(roles)
	t.Cleanup(roleSrv.Close)
	file, err := Parse(yamlWithStorage(product.URL, roleSrv.URL, storage.URL))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CookieSecure: false,
		PublicOrigin: "https://filenkov.store",
		DevHTTP:      true,
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := []byte("test-secret-key-32-bytes-long!!!")
	tok := testToken(t, secret, 15*time.Minute)
	users, err := usersvc.New(roleSrv.URL, roleSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	rej := rejected.New(rdb)
	sessions := sid.New(rdb)
	s, err := New(cfg, file, tok, rej, ratelimit.New(rdb), users, sessions)
	if err != nil {
		t.Fatal(err)
	}
	return &storageHarness{
		gwHarness: &gwHarness{t: t, h: s.Handler(), tok: tok, rej: rej, sessions: sessions, up: productProbe, users: productProbe, secret: secret},
		storage:   storageProbe,
		roles:     roles,
	}
}

func storageReq(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = "storage.filenkov.store"
	return r
}

func TestStorageHealthPublic(t *testing.T) {
	hr := newStorageGW(t)
	rec := hr.do(storageReq(http.MethodGet, "/health"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.storage.path != "/health" {
		t.Fatalf("path %q", hr.storage.path)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("anonymous /health must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestStorageAnonymousRedirectVs401(t *testing.T) {
	hr := newStorageGW(t)
	rec := hr.do(storageReq(http.MethodGet, "/"))
	if rec.Code != http.StatusFound {
		t.Fatalf("html status %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/auth?return_to=") {
		t.Fatalf("location %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("return_to") != "https://storage.filenkov.store/" {
		t.Fatalf("return_to %q", u.Query().Get("return_to"))
	}
	if hr.storage.path != "" {
		t.Fatal("must not proxy")
	}

	xhr := storageReq(http.MethodGet, "/")
	xhr.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec = hr.do(xhr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("xhr %d", rec.Code)
	}

	api := storageReq(http.MethodGet, "/api/files")
	rec = hr.do(api)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/api %d", rec.Code)
	}
}

func TestStorageInjectsHeadersAndForwardsCookie(t *testing.T) {
	hr := newStorageGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "ADMIN")
	hr.roles.role[sub] = "FAMILY"
	r := withAccess(storageReq(http.MethodGet, "/vault"), raw)
	r.AddCookie(&http.Cookie{Name: sid.CookieName, Value: "keep-me-please-not-a-real-sid-but-long-enough"})
	r.Header.Set("X-Auth-Role", "spoof-global")
	r.Header.Set("X-User-Id", "spoof-user")
	r.Header.Set("X-Storage-Role", "ADMIN")
	r.Header.Set("X-Auth-Email", "evil@example.com")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.storage.path != "/vault" {
		t.Fatalf("path %q", hr.storage.path)
	}
	if hr.storage.authID != sub || hr.storage.authEmail != "ada@example.com" || hr.storage.authName != "ada" {
		t.Fatalf("x-auth %+v", hr.storage)
	}
	if hr.storage.authRole != "ADMIN" {
		t.Fatalf("X-Auth-Role must stay hub global_role, got %q", hr.storage.authRole)
	}
	if hr.storage.userID != sub {
		t.Fatalf("X-User-Id %q want %q", hr.storage.userID, sub)
	}
	if hr.storage.storageRole != "FAMILY" {
		t.Fatalf("X-Storage-Role %q want FAMILY (not global_role)", hr.storage.storageRole)
	}
	if !strings.Contains(hr.storage.cookie, sid.CookieName) {
		t.Fatalf("storage must receive auth_session cookie, got %q", hr.storage.cookie)
	}
	if hr.roles.cookie != "" || hr.roles.authz != "" {
		t.Fatalf("User-Service role GET leaked credentials cookie=%q auth=%q", hr.roles.cookie, hr.roles.authz)
	}
	if hr.up.path != "" && hr.up.path == hr.storage.path && hr.up.storageRole != "" {
		// product probe is distinct; hub must not have been the storage target
	}
}

func TestStorageRoleLookupFailure(t *testing.T) {
	hr := newStorageGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "FAMILY")
	hr.roles.fail[sub] = http.StatusInternalServerError
	rec := hr.do(withAccess(storageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("5xx lookup: %d %s", rec.Code, rec.Body.String())
	}
	if hr.storage.path != "" {
		t.Fatal("must not proxy on User-Service 5xx")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "unavailable" {
		t.Fatalf("body %s", rec.Body.String())
	}

	hr.roles.fail[sub] = http.StatusNotFound
	rec = hr.do(withAccess(storageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("404 user: %d %s", rec.Code, rec.Body.String())
	}
	if hr.storage.path != "" {
		t.Fatal("must not proxy on missing user")
	}
}

func TestStorageBlockedHubRoleForbidden(t *testing.T) {
	hr := newStorageGW(t)
	_, raw := hr.login("b@b.c", "blocked", "BLOCKED")
	rec := hr.do(withAccess(storageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
	if hr.storage.path != "" {
		t.Fatal("hub BLOCKED must not reach storage")
	}
}

func TestHubUsersListAdminOnly(t *testing.T) {
	id := uuid.NewString()
	cases := []struct {
		name       string
		role       string
		method     string
		path       string
		wantStatus int
		wantProxy  bool
		wantError  string
	}{
		{"admin GET list", "ADMIN", http.MethodGet, "/users", http.StatusOK, true, ""},
		{"stranger GET list", "STRANGER", http.MethodGet, "/users", http.StatusForbidden, false, "forbidden"},
		{"family GET list", "FAMILY", http.MethodGet, "/users", http.StatusForbidden, false, "forbidden"},
		{"blocked GET list", "BLOCKED", http.MethodGet, "/users", http.StatusForbidden, false, "blocked"},
		{"anonymous GET list", "", http.MethodGet, "/users", http.StatusUnauthorized, false, "unauthorized"},
		{"admin POST create", "ADMIN", http.MethodPost, "/users", http.StatusNotFound, false, "not found"},
		{"admin GET by-google-sub", "ADMIN", http.MethodGet, "/users/by-google-sub/11001", http.StatusNotFound, false, "not found"},
		{"stranger GET by id", "STRANGER", http.MethodGet, "/users/" + id, http.StatusOK, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hr := newGW(t)
			r := req(tc.method, tc.path)
			if tc.role != "" {
				_, raw := hr.login("ada@example.com", "ada", tc.role)
				r = withAccess(r, raw)
			}
			rec := hr.do(r)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d want %d body %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantError != "" {
				var body map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != tc.wantError {
					t.Fatalf("error body %s want %q", rec.Body.String(), tc.wantError)
				}
			}
			if tc.wantProxy {
				if hr.users.path != tc.path {
					t.Fatalf("upstream path %q want %q", hr.users.path, tc.path)
				}
				if hr.up.path != "" {
					t.Fatalf("must not hit product upstream, path=%q", hr.up.path)
				}
			} else if hr.users.path != "" {
				t.Fatalf("must not proxy, path=%q", hr.users.path)
			}
		})
	}
}

func TestHubUsersListProxiesWithXUserId(t *testing.T) {
	hr := newGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "ADMIN")
	r := withAccess(req(http.MethodGet, "/users"), raw)
	r.Header.Set("Authorization", "Bearer "+raw)
	r.Header.Set("X-User-Id", "client-spoof")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.users.path != "/users" {
		t.Fatalf("path %q", hr.users.path)
	}
	if hr.users.userID != sub {
		t.Fatalf("X-User-Id %q want %q", hr.users.userID, sub)
	}
	if hr.users.cookie != "" {
		t.Fatalf("cookie forwarded: %q", hr.users.cookie)
	}
	if hr.users.authorization != "" {
		t.Fatalf("Authorization forwarded: %q", hr.users.authorization)
	}
	if hr.users.authID != "" || hr.users.authRole != "" {
		t.Fatalf("X-Auth-* must not go to User-Service: %+v", hr.users)
	}
	hit, err := hr.rej.IsRejected(context.Background(), "", sub, time.Now().Add(-time.Second))
	if err != nil || hit {
		t.Fatalf("GET /users must not write rejected:user: hit=%v err=%v", hit, err)
	}
}

func yamlWithMessanger(product, userSvc, messanger string) []byte {
	return []byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
timeouts:
  proxy: 60s
upstreams:
  api: "` + product + `"
  web: "` + product + `"
  hermes: "` + product + `"
  user_service: "` + userSvc + `"
  messanger: "` + messanger + `"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /users/by-email/*
        auth: reject
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
      - match: /*
        upstream: web
        auth: public
  messanger.filenkov.store:
    role_field: global_role
    timeouts:
      proxy: 5m
    routes:
      - match: /health
        upstream: messanger
        auth: public
      - match: /*
        upstream: messanger
        auth: session
`)
}

type messangerHarness struct {
	*gwHarness
	messanger *probe
	roles     *storageRoleMock
}

func newMessangerGW(t *testing.T) *messangerHarness {
	t.Helper()
	product, productProbe := newUpstream(t)
	messanger, messangerProbe := newUpstream(t)
	roles := newStorageRoleMock()
	roleSrv := httptest.NewServer(roles)
	t.Cleanup(roleSrv.Close)
	file, err := Parse(yamlWithMessanger(product.URL, roleSrv.URL, messanger.URL))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CookieSecure: false,
		PublicOrigin: "https://filenkov.store",
		DevHTTP:      true,
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := []byte("test-secret-key-32-bytes-long!!!")
	tok := testToken(t, secret, 15*time.Minute)
	users, err := usersvc.New(roleSrv.URL, roleSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	rej := rejected.New(rdb)
	sessions := sid.New(rdb)
	s, err := New(cfg, file, tok, rej, ratelimit.New(rdb), users, sessions)
	if err != nil {
		t.Fatal(err)
	}
	return &messangerHarness{
		gwHarness: &gwHarness{t: t, h: s.Handler(), tok: tok, rej: rej, sessions: sessions, up: productProbe, users: productProbe, secret: secret},
		messanger: messangerProbe,
		roles:     roles,
	}
}

func messangerReq(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = "messanger.filenkov.store"
	return r
}

func TestMessangerHealthPublic(t *testing.T) {
	hr := newMessangerGW(t)
	rec := hr.do(messangerReq(http.MethodGet, "/health"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.messanger.path != "/health" {
		t.Fatalf("path %q", hr.messanger.path)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("anonymous /health must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestMessangerAnonymousRedirectVs401(t *testing.T) {
	hr := newMessangerGW(t)
	rec := hr.do(messangerReq(http.MethodGet, "/"))
	if rec.Code != http.StatusFound {
		t.Fatalf("html status %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/auth?return_to=") {
		t.Fatalf("location %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("return_to") != "https://messanger.filenkov.store/" {
		t.Fatalf("return_to %q", u.Query().Get("return_to"))
	}
	if hr.messanger.path != "" {
		t.Fatal("must not proxy")
	}

	xhr := messangerReq(http.MethodGet, "/")
	xhr.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec = hr.do(xhr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("xhr %d", rec.Code)
	}

	api := messangerReq(http.MethodGet, "/api/dialogs")
	rec = hr.do(api)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/api %d", rec.Code)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("anonymous requests must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestMessangerInjectsHeadersNoRoleLookup(t *testing.T) {
	hr := newMessangerGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "FAMILY")
	r := withAccess(messangerReq(http.MethodGet, "/api/dialogs"), raw)
	r.Header.Set("X-Auth-User-Id", "spoof-user")
	r.Header.Set("X-Auth-Email", "evil@example.com")
	r.Header.Set("X-Auth-Name", "evil")
	r.Header.Set("X-Auth-Role", "ADMIN")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if hr.messanger.path != "/api/dialogs" {
		t.Fatalf("path %q", hr.messanger.path)
	}
	if hr.messanger.authID != sub {
		t.Fatalf("X-Auth-User-Id %q want %q", hr.messanger.authID, sub)
	}
	if hr.messanger.authEmail != "ada@example.com" || hr.messanger.authName != "ada" {
		t.Fatalf("x-auth %+v", hr.messanger)
	}
	if hr.messanger.authRole != "FAMILY" {
		t.Fatalf("X-Auth-Role must be the hub global_role, got %q", hr.messanger.authRole)
	}
	if hr.messanger.storageRole != "" || hr.messanger.userID != "" {
		t.Fatalf("global_role host must not inject X-User-Id/X-Storage-Role: %+v", hr.messanger)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("messanger host must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestMessangerBlockedForbidden(t *testing.T) {
	hr := newMessangerGW(t)
	_, raw := hr.login("b@b.c", "blocked", "BLOCKED")
	rec := hr.do(withAccess(messangerReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
	if hr.messanger.path != "" {
		t.Fatal("hub BLOCKED must not reach the messanger")
	}
	if hr.roles.calls != 0 {
		t.Fatalf("BLOCKED must be stopped before any User-Service call, calls=%d", hr.roles.calls)
	}
}

func TestStorageProxyTimeoutOverride(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)
	roles := newStorageRoleMock()
	roleSrv := httptest.NewServer(roles)
	t.Cleanup(roleSrv.Close)
	file, err := Parse([]byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
timeouts:
  proxy: 60s
upstreams:
  storage: "` + slow.URL + `"
hosts:
  storage.filenkov.store:
    role_field: storage
    timeouts:
      proxy: 50ms
    routes:
      - match: /*
        upstream: storage
        auth: session
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{PublicOrigin: "https://filenkov.store", DevHTTP: true}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	tok := testToken(t, []byte("test-secret-key-32-bytes-long!!!"), time.Minute)
	users, err := usersvc.New(roleSrv.URL, roleSrv.Client())
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg, file, tok, rejected.New(rdb), ratelimit.New(rdb), users, nil)
	if err != nil {
		t.Fatal(err)
	}
	hr := &gwHarness{t: t, h: s.Handler(), tok: tok}
	sub, raw := hr.login("a@b.c", "a", "STRANGER")
	roles.role[sub] = "STRANGER"
	rec := hr.do(withAccess(storageReq(http.MethodGet, "/upload"), raw))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502 from per-host proxy timeout, got %d %s", rec.Code, rec.Body.String())
	}
}

func yamlWithImage(product, userSvc, image string) []byte {
	return []byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
timeouts:
  proxy: 60s
upstreams:
  api: "` + product + `"
  web: "` + product + `"
  user_service: "` + userSvc + `"
  qwen_image: "` + image + `"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /users/*
        upstream: user_service
        auth: session
        preserve_host: false
      - match: /*
        upstream: web
        auth: public
  image.filenkov.store:
    role_field: image
    timeouts:
      proxy: 30m
    routes:
      - match: /healthz
        upstream: qwen_image
        auth: public
      - match: /*
        upstream: qwen_image
        auth: session
`)
}

type imageHarness struct {
	*gwHarness
	image *probe
	roles *storageRoleMock
}

func newImageGW(t *testing.T) *imageHarness {
	t.Helper()
	return newImageGWUsers(t, true)
}

// newImageGWUsers builds the image harness; withUsers=false leaves the
// User-Service client nil (not configured).
func newImageGWUsers(t *testing.T, withUsers bool) *imageHarness {
	t.Helper()
	product, productProbe := newUpstream(t)
	image, imageProbe := newUpstream(t)
	roles := newServiceRoleMock("image")
	roleSrv := httptest.NewServer(roles)
	t.Cleanup(roleSrv.Close)
	file, err := Parse(yamlWithImage(product.URL, roleSrv.URL, image.URL))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CookieSecure: false,
		PublicOrigin: "https://filenkov.store",
		DevHTTP:      true,
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := []byte("test-secret-key-32-bytes-long!!!")
	tok := testToken(t, secret, 15*time.Minute)
	var users *usersvc.Client
	if withUsers {
		users, err = usersvc.New(roleSrv.URL, roleSrv.Client())
		if err != nil {
			t.Fatal(err)
		}
	}
	rej := rejected.New(rdb)
	sessions := sid.New(rdb)
	s, err := New(cfg, file, tok, rej, ratelimit.New(rdb), users, sessions)
	if err != nil {
		t.Fatal(err)
	}
	return &imageHarness{
		gwHarness: &gwHarness{t: t, h: s.Handler(), tok: tok, rej: rej, sessions: sessions, up: productProbe, users: productProbe, secret: secret},
		image:     imageProbe,
		roles:     roles,
	}
}

func imageReq(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Host = "image.filenkov.store"
	return r
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != want {
		t.Fatalf("body %s want error %q", rec.Body.String(), want)
	}
}

func TestHealthzLocalUnlessHostRoutesIt(t *testing.T) {
	hr := newImageGW(t)

	rec := hr.do(imageReq(http.MethodGet, "/healthz"))
	if rec.Code != http.StatusOK {
		t.Fatalf("image healthz status %d %s", rec.Code, rec.Body.String())
	}
	if hr.image.path != "/healthz" {
		t.Fatalf("image /healthz must be proxied to the app, upstream path %q", hr.image.path)
	}

	for _, host := range []string{"filenkov.store", "0.0.0.0:8083"} {
		hr.up.path = ""
		r := req(http.MethodGet, "/healthz")
		r.Host = host
		rec = hr.do(r)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s healthz status %d", host, rec.Code)
		}
		if hr.up.path != "" {
			t.Fatalf("%s: catch-all must not claim /healthz, upstream path %q", host, hr.up.path)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
			t.Fatalf("%s: local healthz body %s", host, rec.Body.String())
		}
	}
}

func TestImageHealthzPublicNoRoleLookup(t *testing.T) {
	hr := newImageGW(t)
	rec := hr.do(imageReq(http.MethodGet, "/healthz"))
	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous status %d %s", rec.Code, rec.Body.String())
	}
	if hr.image.path != "/healthz" {
		t.Fatalf("path %q", hr.image.path)
	}

	hr.image.path = ""
	_, raw := hr.login("s@b.c", "stranger", "STRANGER")
	rec = hr.do(withAccess(imageReq(http.MethodGet, "/healthz"), raw))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed-in STRANGER healthz status %d %s", rec.Code, rec.Body.String())
	}
	if hr.image.path != "/healthz" {
		t.Fatalf("signed-in STRANGER healthz must be proxied, path %q", hr.image.path)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("public /healthz must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestImageHealthzWithoutUserService(t *testing.T) {
	hr := newImageGWUsers(t, false)
	_, raw := hr.login("f@b.c", "fam", "FAMILY")
	rec := hr.do(withAccess(imageReq(http.MethodGet, "/healthz"), raw))
	if rec.Code != http.StatusOK || hr.image.path != "/healthz" {
		t.Fatalf("healthz must not depend on User-Service: %d path=%q", rec.Code, hr.image.path)
	}

	hr.image.path = ""
	rec = hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil users client: %d %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, "unavailable")
	if hr.image.path != "" {
		t.Fatal("must not proxy without a User-Service client")
	}
}

func TestImageAnonymousRedirectVs401(t *testing.T) {
	hr := newImageGW(t)
	rec := hr.do(imageReq(http.MethodGet, "/"))
	if rec.Code != http.StatusFound {
		t.Fatalf("html status %d", rec.Code)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rec.Header().Get("Location"), "https://filenkov.store/auth?return_to=") {
		t.Fatalf("location %q", rec.Header().Get("Location"))
	}
	if u.Query().Get("return_to") != "https://image.filenkov.store/" {
		t.Fatalf("return_to %q", u.Query().Get("return_to"))
	}

	xhr := imageReq(http.MethodGet, "/")
	xhr.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec = hr.do(xhr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("xhr %d", rec.Code)
	}
	assertErrorBody(t, rec, "unauthorized")

	rec = hr.do(imageReq(http.MethodPost, "/api/generate"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/generate %d", rec.Code)
	}
	if hr.image.path != "" {
		t.Fatalf("anonymous must not proxy, path %q", hr.image.path)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("anonymous requests must not call User-Service, calls=%d", hr.roles.calls)
	}
}

func TestImageAllowedRolesProxied(t *testing.T) {
	for _, role := range []string{"FAMILY", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			hr := newImageGW(t)
			sub, raw := hr.login("ada@example.com", "ada", "STRANGER")
			hr.roles.role[sub] = role
			r := withAccess(imageReq(http.MethodPost, "/api/generate"), raw)
			r.Header.Set("X-Auth-User-Id", "spoof-user")
			r.Header.Set("X-Auth-Email", "evil@example.com")
			r.Header.Set("X-Auth-Name", "evil")
			r.Header.Set("X-Auth-Role", "ADMIN")
			r.Header.Set("X-User-Id", "spoof-user")
			r.Header.Set("X-Storage-Role", "ADMIN")
			rec := hr.do(r)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			if hr.image.method != http.MethodPost || hr.image.path != "/api/generate" {
				t.Fatalf("upstream %s %q", hr.image.method, hr.image.path)
			}
			if hr.image.authID != sub || hr.image.authEmail != "ada@example.com" || hr.image.authName != "ada" {
				t.Fatalf("x-auth %+v", hr.image)
			}
			if hr.image.authRole != "STRANGER" {
				t.Fatalf("X-Auth-Role must be the hub global_role, got %q", hr.image.authRole)
			}
			if hr.image.userID != "" || hr.image.storageRole != "" || hr.image.authorization != "" {
				t.Fatalf("image host must get only X-Auth-*: %+v", hr.image)
			}
			if hr.roles.lastPath != "/users/"+sub+"/roles/image" {
				t.Fatalf("role lookup path %q", hr.roles.lastPath)
			}
			if hr.roles.cookie != "" || hr.roles.authz != "" {
				t.Fatalf("User-Service role GET leaked credentials cookie=%q auth=%q", hr.roles.cookie, hr.roles.authz)
			}
		})
	}
}

func TestImageDeniedRolesForbidden(t *testing.T) {
	cases := []struct {
		name    string
		hubRole string
		role    string // "" = no row, served as STRANGER
	}{
		{"no row", "FAMILY", ""},
		{"stranger", "FAMILY", "STRANGER"},
		{"blocked", "FAMILY", "BLOCKED"},
		{"unknown value", "FAMILY", "OWNER"},
		{"hub admin without image row", "ADMIN", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hr := newImageGW(t)
			sub, raw := hr.login("ada@example.com", "ada", tc.hubRole)
			if tc.role != "" {
				hr.roles.role[sub] = tc.role
			}
			rec := hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status %d %s", rec.Code, rec.Body.String())
			}
			assertErrorBody(t, rec, "forbidden")
			if hr.image.path != "" {
				t.Fatalf("must not proxy, path %q", hr.image.path)
			}
			if hr.roles.calls != 1 {
				t.Fatalf("role lookup calls=%d want 1", hr.roles.calls)
			}
		})
	}
}

func TestImageHubBlockedAndRejectedForbidden(t *testing.T) {
	hr := newImageGW(t)
	_, raw := hr.login("b@b.c", "blocked", "BLOCKED")
	rec := hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hub BLOCKED status %d", rec.Code)
	}

	sub, raw := hr.login("r@b.c", "revoked", "FAMILY")
	hr.roles.role[sub] = "FAMILY"
	if err := hr.rej.RejectUser(context.Background(), sub, time.Hour); err != nil {
		t.Fatal(err)
	}
	rec = hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("rejected_cache user status %d", rec.Code)
	}
	if hr.image.path != "" {
		t.Fatalf("must not proxy, path %q", hr.image.path)
	}
	if hr.roles.calls != 0 {
		t.Fatalf("hub checks must stop before any User-Service call, calls=%d", hr.roles.calls)
	}
}

func TestImageRoleLookupFailure(t *testing.T) {
	hr := newImageGW(t)
	sub, raw := hr.login("ada@example.com", "ada", "FAMILY")
	hr.roles.role[sub] = "FAMILY"
	hr.roles.fail[sub] = http.StatusInternalServerError
	rec := hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("5xx lookup: %d %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, "unavailable")
	if hr.image.path != "" {
		t.Fatal("must not proxy on User-Service 5xx")
	}

	hr.roles.fail[sub] = http.StatusNotFound
	rec = hr.do(withAccess(imageReq(http.MethodGet, "/"), raw))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("404 user: %d %s", rec.Code, rec.Body.String())
	}
	assertErrorBody(t, rec, "unauthorized")
	if hr.image.path != "" {
		t.Fatal("must not proxy on missing user")
	}
}

func TestServiceRouteInjectsTokenAndOverridesClient(t *testing.T) {
	t.Setenv("SWITCH_TOKEN", "switch-secret")
	product, productProbe := newUpstream(t)
	userSvc, userProbe := newUpstream(t)
	file, err := Parse([]byte(`
listen: ":0"
public_origin: "https://filenkov.store"
rate_limit:
  requests_per_minute: 10000
upstreams:
  api: "` + product.URL + `"
  user_service: "` + userSvc.URL + `"
hosts:
  filenkov.store:
    role_field: global_role
    routes:
      - match: /api/services
        upstream: api
        auth: admin
        headers:
          X-Service-Token: "${SWITCH_TOKEN}"
      - match: /api/services/*
        upstream: api
        auth: admin
        headers:
          X-Service-Token: "${SWITCH_TOKEN}"
      - match: /api/*
        upstream: api
        auth: session
`))
	if err != nil {
		t.Fatal(err)
	}
	hr := newGWFromFile(t, file, productProbe, userProbe)

	// Anonymous stays out: admin route, no session.
	if rec := hr.do(req(http.MethodGet, "/api/services")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous /api/services = %d, want 401", rec.Code)
	}

	_, raw := hr.login("boss@b.c", "boss", "ADMIN")
	r := withAccess(req(http.MethodPost, "/api/services/ocr/state"), raw)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Service-Token", "client-spoof")
	rec := hr.do(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if productProbe.path != "/api/services/ocr/state" {
		t.Fatalf("upstream path %q", productProbe.path)
	}
	if productProbe.serviceToken != "switch-secret" {
		t.Fatalf("X-Service-Token %q, want the configured value", productProbe.serviceToken)
	}
	if productProbe.authEmail != "boss@b.c" {
		t.Fatalf("X-Auth-Email %q", productProbe.authEmail)
	}
}
