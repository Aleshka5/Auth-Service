package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
)

const refreshTimeout = 10 * time.Second

// silentRefresh asks the Auth API to rotate cookies when a protected route
// has no usable access JWT but the browser still sent auth_refresh.
// The gateway does not mint tokens. Parallel requests share one refresh so
// rotation does not invalidate the cookie out from under the others.
//
// ok means the request cookie jar now carries a new access JWT.
// status is 0 when refresh was skipped. 401 falls through to the anonymous
// deny path after any clearing Set-Cookie is copied. Other statuses are terminal.
func (s *Server) silentRefresh(w http.ResponseWriter, r *http.Request) (ok bool, status int) {
	refreshCookie, err := r.Cookie(token.CookieRefresh)
	if err != nil || refreshCookie.Value == "" {
		return false, 0
	}
	if s.file == nil || strings.TrimSpace(s.file.Upstreams["api"]) == "" || s.refreshClient == nil {
		slog.Error("gateway_refresh", "err", "api upstream is not configured")
		return false, http.StatusServiceUnavailable
	}

	session := ""
	if c, err := r.Cookie(sid.CookieName); err == nil {
		session = c.Value
	}
	out := s.refreshFlights.do(refreshKey(refreshCookie.Value), func() refreshOutcome {
		return s.postRefresh(r.Context(), refreshCookie.Value, session, clientIP(r))
	})
	if out.err != nil {
		slog.Error("gateway_refresh", "err", out.err, "path", r.URL.Path, "host", r.Host)
		return false, http.StatusServiceUnavailable
	}

	switch out.status {
	case http.StatusOK:
		access := cookieValue(out.cookies, token.CookieAccess)
		if access == "" {
			slog.Error("gateway_refresh", "err", "refresh response missing access cookie", "path", r.URL.Path)
			return false, http.StatusServiceUnavailable
		}
		addResponseCookies(w, out.cookies)
		setRequestCookies(r, map[string]string{
			token.CookieAccess:  access,
			token.CookieRefresh: cookieValue(out.cookies, token.CookieRefresh),
			sid.CookieName:      cookieValue(out.cookies, sid.CookieName),
		})
		slog.Info("gateway_refresh", "path", r.URL.Path, "host", r.Host)
		return true, http.StatusOK
	case http.StatusUnauthorized:
		addResponseCookies(w, out.cookies)
		return false, http.StatusUnauthorized
	case http.StatusForbidden:
		addResponseCookies(w, out.cookies)
		slog.Warn("gateway_refresh_blocked", "path", r.URL.Path, "host", r.Host)
		return false, http.StatusForbidden
	case http.StatusTooManyRequests:
		slog.Warn("gateway_refresh_rate_limit", "path", r.URL.Path, "host", r.Host)
		return false, http.StatusTooManyRequests
	default:
		slog.Error("gateway_refresh", "status", out.status, "path", r.URL.Path, "host", r.Host)
		return false, http.StatusServiceUnavailable
	}
}

func routeNeedsAccess(policy string) bool {
	return policy == PolicySession || policy == PolicyAdmin
}

type refreshOutcome struct {
	status  int
	cookies []*http.Cookie
	err     error
}

func (s *Server) postRefresh(parent context.Context, refreshVal, sessionVal, ip string) refreshOutcome {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), refreshTimeout)
	defer cancel()

	endpoint := strings.TrimRight(s.file.Upstreams["api"], "/") + "/api/refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return refreshOutcome{err: err}
	}
	req.AddCookie(&http.Cookie{Name: token.CookieRefresh, Value: refreshVal})
	if sessionVal != "" {
		req.AddCookie(&http.Cookie{Name: sid.CookieName, Value: sessionVal})
	}
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}

	resp, err := s.refreshClient.Do(req)
	if err != nil {
		return refreshOutcome{err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return refreshOutcome{status: resp.StatusCode, cookies: resp.Cookies()}
}

func newRefreshClient() *http.Client {
	return &http.Client{
		Timeout: refreshTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type refreshFlights struct {
	mu    sync.Mutex
	calls map[string]*refreshCall
}

type refreshCall struct {
	done   chan struct{}
	result refreshOutcome
}

func (f *refreshFlights) do(key string, fn func() refreshOutcome) refreshOutcome {
	if f == nil {
		return fn()
	}
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]*refreshCall{}
	}
	if c, ok := f.calls[key]; ok {
		f.mu.Unlock()
		<-c.done
		return c.result
	}
	c := &refreshCall{done: make(chan struct{})}
	f.calls[key] = c
	f.mu.Unlock()

	c.result = fn()
	close(c.done)

	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	return c.result
}

func refreshKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, c := range cookies {
		if c != nil && c.Name == name {
			return c.Value
		}
	}
	return ""
}

func addResponseCookies(w http.ResponseWriter, cookies []*http.Cookie) {
	for _, c := range cookies {
		if c == nil {
			continue
		}
		switch c.Name {
		case token.CookieAccess, token.CookieRefresh, sid.CookieName:
			http.SetCookie(w, c)
		}
	}
}

func setRequestCookies(r *http.Request, updates map[string]string) {
	existing := r.Cookies()
	r.Header.Del("Cookie")
	seen := map[string]bool{}
	for _, c := range existing {
		if v, ok := updates[c.Name]; ok && v != "" {
			c.Value = v
			seen[c.Name] = true
		}
		r.AddCookie(c)
	}
	for name, v := range updates {
		if v != "" && !seen[name] {
			r.AddCookie(&http.Cookie{Name: name, Value: v})
		}
	}
}
