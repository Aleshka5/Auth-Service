package gateway

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aleshka5/auth-service/internal/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestSilentRefreshProxiesWithNewAccess(t *testing.T) {
	hr := newGW(t)
	sub := uuid.NewString()
	var refreshes atomic.Int32
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/refresh" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		refreshes.Add(1)
		if r.Header.Get("X-Forwarded-For") == "" {
			t.Errorf("refresh missing client IP")
		}
		if got := r.Header.Get("Cookie"); got == "" || !containsCookie(got, token.CookieRefresh) {
			t.Errorf("refresh cookie: %q", got)
		}
		access, _, err := hr.tok.IssueAccess(sub, "ada@x.c", "ada", "ADMIN")
		if err != nil {
			t.Error(err)
			http.Error(w, "issue", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: token.CookieAccess, Value: access, Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: token.CookieRefresh, Value: "rotated-refresh", Path: "/", HttpOnly: true})
		w.WriteHeader(http.StatusOK)
	})

	req := withRefresh(withAccess(req(http.MethodGet, "/admin"), expiredAccess(t, hr)), "still-valid")
	rec := hr.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes %d", refreshes.Load())
	}
	if hr.up.authID != sub || hr.up.authRole != "ADMIN" || hr.up.authName != "ada" {
		t.Fatalf("identity %+v", hr.up)
	}
	if hr.up.path != "/admin" {
		t.Fatalf("proxied path %q", hr.up.path)
	}
	if !setCookieNamed(rec, token.CookieAccess) || !setCookieNamed(rec, token.CookieRefresh) {
		t.Fatalf("set-cookie %q", rec.Header().Values("Set-Cookie"))
	}
}

func TestSilentRefreshSingleFlight(t *testing.T) {
	hr := newGW(t)
	sub := uuid.NewString()
	var refreshes atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/refresh" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		n := refreshes.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		access, _, err := hr.tok.IssueAccess(sub, "ada@x.c", "ada", "STRANGER")
		if err != nil {
			http.Error(w, "issue", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: token.CookieAccess, Value: access, Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: token.CookieRefresh, Value: "rotated", Path: "/"})
		w.WriteHeader(http.StatusOK)
	})

	const n = 8
	expired := expiredAccess(t, hr)
	var wg sync.WaitGroup
	codes := make([]int, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			r := withRefresh(withAccess(req(http.MethodGet, "/api/me"), expired), "same-refresh")
			rec := hr.do(r)
			codes[i] = rec.Code
		}(i)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not start")
	}
	close(release)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for refreshed requests")
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes %d", refreshes.Load())
	}
	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("req %d status %d", i, code)
		}
	}
}

func TestSilentRefreshUnauthorizedFallsThrough(t *testing.T) {
	hr := newGW(t)
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/refresh" {
			t.Errorf("proxied %s", r.URL.Path)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: token.CookieAccess, Value: "", Path: "/", MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: token.CookieRefresh, Value: "", Path: "/", MaxAge: -1})
		w.WriteHeader(http.StatusUnauthorized)
	})
	rec := hr.do(withRefresh(withAccess(req(http.MethodGet, "/api/me"), expiredAccess(t, hr)), "dead"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if !setCookieNamed(rec, token.CookieRefresh) {
		t.Fatalf("expected clearing set-cookie, got %q", rec.Header().Values("Set-Cookie"))
	}
}

func TestSilentRefreshBlocked(t *testing.T) {
	hr := newGW(t)
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/refresh" {
			t.Errorf("proxied %s", r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	rec := hr.do(withRefresh(req(http.MethodGet, "/admin"), "refresh"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSilentRefreshUpstreamDown(t *testing.T) {
	hr := newGW(t)
	var refreshes atomic.Int32
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/refresh" {
			refreshes.Add(1)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	rec := hr.do(withRefresh(req(http.MethodGet, "/"), "refresh"))
	if rec.Code != http.StatusOK {
		t.Fatalf("public route status %d", rec.Code)
	}
	if refreshes.Load() != 0 {
		t.Fatalf("public route refreshed %d times", refreshes.Load())
	}
	rec = hr.do(withRefresh(req(http.MethodGet, "/api/me"), "refresh"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes %d", refreshes.Load())
	}
}

func TestValidAccessSkipsRefresh(t *testing.T) {
	hr := newGW(t)
	var refreshes atomic.Int32
	hr.up.setResponder(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/refresh" {
			refreshes.Add(1)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	_, raw := hr.login("ada@x.c", "ada", "STRANGER")
	rec := hr.do(withRefresh(withAccess(req(http.MethodGet, "/api/me"), raw), "refresh"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if refreshes.Load() != 0 {
		t.Fatalf("refreshed a live access token")
	}
}

func expiredAccess(t *testing.T, hr *gwHarness) string {
	t.Helper()
	expiredAt := time.Now().Add(-time.Hour)
	claims := token.AccessClaims{
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
		Email:      "old@x.c",
		Username:   "old",
		GlobalRole: "ADMIN",
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(hr.secret)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func withRefresh(r *http.Request, refresh string) *http.Request {
	r.AddCookie(&http.Cookie{Name: token.CookieRefresh, Value: refresh})
	return r
}

func containsCookie(header, name string) bool {
	req := &http.Request{Header: http.Header{"Cookie": {header}}}
	c, err := req.Cookie(name)
	return err == nil && c.Value != ""
}

func setCookieNamed(rec *httptest.ResponseRecorder, name string) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}
