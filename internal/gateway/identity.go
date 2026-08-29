package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
	"github.com/google/uuid"
)

type accessIdentity struct {
	claims      *token.AccessClaims
	userRevoked bool
	storageRole string
}

func (s *Server) lookupAccess(r *http.Request) (accessIdentity, error) {
	c, err := r.Cookie(token.CookieAccess)
	if err != nil || c.Value == "" {
		return accessIdentity{}, nil
	}
	if s.tokens == nil {
		return accessIdentity{}, nil
	}
	claims, err := s.tokens.ParseAccess(c.Value)
	if err != nil {
		return accessIdentity{}, nil
	}
	if s.rej == nil {
		return accessIdentity{claims: claims}, nil
	}
	jtiHit, err := s.rej.IsRejected(r.Context(), claims.JTI(), "", claims.IssuedAtTime())
	if err != nil {
		return accessIdentity{}, err
	}
	if jtiHit {
		return accessIdentity{}, nil
	}
	userHit, err := s.rej.IsRejected(r.Context(), "", claims.Sub(), claims.IssuedAtTime())
	if err != nil {
		return accessIdentity{}, err
	}
	if userHit {
		return accessIdentity{claims: claims, userRevoked: true}, nil
	}
	return accessIdentity{claims: claims}, nil
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request, policy string, id accessIdentity) bool {
	if policy == PolicyPublic {
		return true
	}
	if id.claims == nil {
		s.denyAnonymous(w, r)
		return false
	}
	if id.userRevoked || id.claims.GlobalRole == roleBlocked {
		slog.Warn("gateway_blocked", "user_id", id.claims.Sub(), "path", r.URL.Path, "host", r.Host)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "blocked"})
		return false
	}
	if policy == PolicyAdmin && id.claims.GlobalRole != roleAdmin {
		slog.Warn("gateway_forbidden", "user_id", id.claims.Sub(), "path", r.URL.Path, "host", r.Host, "role", id.claims.GlobalRole)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return false
	}
	return true
}

func stripIncomingAuth(h http.Header) {
	for k := range h {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-auth-") || lk == "x-user-id" || lk == "x-storage-role" {
			h.Del(k)
		}
	}
}

func injectIdentity(h http.Header, claims *token.AccessClaims) {
	if claims == nil {
		return
	}
	h.Set("X-Auth-User-Id", claims.Sub())
	h.Set("X-Auth-Email", claims.Email)
	h.Set("X-Auth-Name", claims.Username)
	h.Set("X-Auth-Role", claims.GlobalRole)
}

func stripUserServiceSecrets(out *http.Request) {
	out.Header.Del("Cookie")
	out.Header.Del("Authorization")
	if out.URL == nil {
		return
	}
	q := out.URL.Query()
	changed := false
	for _, k := range []string{"access_token", "refresh_token", "id_token", "token", "jwt", token.CookieAccess, token.CookieRefresh, sid.CookieName} {
		if q.Has(k) {
			q.Del(k)
			changed = true
		}
	}
	if changed {
		out.URL.RawQuery = q.Encode()
	}
}

func (s *Server) applyProxyIdentity(pr *httputil.ProxyRequest, t proxyTarget) {
	if t.route == nil {
		return
	}
	if t.route.upstreamName == upstreamUserService {
		stripUserServiceSecrets(pr.Out)
		if t.id.claims != nil {
			pr.Out.Header.Set("X-User-Id", t.id.claims.Sub())
		}
		return
	}
	if t.id.claims != nil {
		injectIdentity(pr.Out.Header, t.id.claims)
		if t.host != nil && t.host.roleField == "storage" {
			pr.Out.Header.Set("X-User-Id", t.id.claims.Sub())
			if t.id.storageRole != "" {
				pr.Out.Header.Set("X-Storage-Role", t.id.storageRole)
			}
		}
		if t.route.upstreamName == upstreamHermes {
			injectHermesDashboardAuth(pr, t)
		}
	}
}

// injectHermesDashboardAuth gives Hermes a DashboardAuthProvider credential.
// Hermes refuses non-loopback binds unless a provider is registered, and its
// gated middleware authenticates HTTP via Authorization Bearer (then session
// cookies). The browser never sees this header; we copy the already-validated
// access cookie so Hermes can verify the same HS256 JWT. User-Service still
// never receives a JWT.
func injectHermesDashboardAuth(pr *httputil.ProxyRequest, t proxyTarget) {
	if c, err := pr.In.Cookie(token.CookieAccess); err == nil && strings.TrimSpace(c.Value) != "" {
		pr.Out.Header.Set("Authorization", "Bearer "+c.Value)
	}
	if prefix := strings.TrimSpace(t.route.stripPrefix); prefix != "" {
		pr.Out.Header.Set("X-Forwarded-Prefix", prefix)
	}
}

func (s *Server) attachStorageRole(ctx context.Context, host *compiledHost, id *accessIdentity) (status int, msg string, ok bool) {
	if host == nil || host.roleField != "storage" || id == nil || id.claims == nil {
		return 0, "", true
	}
	if s.users == nil {
		slog.Error("gateway_storage_role", "err", "user-service client is not configured")
		return http.StatusServiceUnavailable, "unavailable", false
	}
	sr, err := s.users.GetServiceRole(ctx, id.claims.Sub(), "storage")
	if err != nil {
		if usersvc.IsNotFound(err) {
			slog.Warn("gateway_storage_role_missing_user", "user_id", id.claims.Sub())
			return http.StatusUnauthorized, "unauthorized", false
		}
		slog.Error("gateway_storage_role", "err", err, "user_id", id.claims.Sub())
		return http.StatusServiceUnavailable, "unavailable", false
	}
	id.storageRole = string(sr.Role)
	return 0, "", true
}

func (s *Server) maybeRejectTargetUser(resp *http.Response) {
	if s.rej == nil || resp == nil || resp.Request == nil {
		return
	}
	if resp.StatusCode != http.StatusOK || resp.Request.Method != http.MethodPut {
		return
	}
	t, _ := resp.Request.Context().Value(proxyTargetKey).(proxyTarget)
	if t.route == nil || t.route.upstreamName != upstreamUserService {
		return
	}
	id := targetUserID(resp.Request.URL.Path)
	if id == "" {
		return
	}
	if err := s.rej.RejectUser(resp.Request.Context(), id, s.rejectUserTTL()); err != nil {
		slog.Error("gateway_reject_user", "err", err, "user_id", id, "path", resp.Request.URL.Path)
	}
	if s.sessions != nil {
		if err := s.sessions.DeleteByUser(resp.Request.Context(), id); err != nil {
			slog.Error("gateway_delete_session", "err", err, "user_id", id)
		}
	}
}

func (s *Server) rejectUserTTL() time.Duration {
	if s.tokens != nil {
		if ttl := s.tokens.RefreshTTL(); ttl > 0 {
			return ttl
		}
	}
	return 168 * time.Hour
}

func targetUserID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "users" {
		return ""
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return ""
	}
	switch {
	case len(parts) == 3 && parts[2] == "role":
		return parts[1]
	case len(parts) == 4 && parts[2] == "roles":
		return parts[1]
	default:
		return ""
	}
}

func (s *Server) denyAnonymous(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	returnTo := s.safeReturnTo(r)
	loc := strings.TrimRight(s.cfg.PublicOrigin, "/") + "/auth?return_to=" + url.QueryEscape(returnTo)
	http.Redirect(w, r, loc, http.StatusFound)
}

func (s *Server) safeReturnTo(r *http.Request) string {
	allowed := s.file.AllowedHostsMap()
	candidates := []string{
		forwardedProto(r, s.cfg.DevHTTP) + "://" + r.Host + r.URL.RequestURI(),
		"https://" + r.Host + r.URL.RequestURI(),
	}
	for _, raw := range candidates {
		resolved, err := oauth.ResolveReturnTo(raw, s.cfg.PublicOrigin, allowed, s.cfg.DevHTTP)
		if err == nil {
			return resolved
		}
	}
	return s.cfg.PublicOrigin
}

func wantsJSON(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/users/") || r.URL.Path == "/users" {
		return true
	}
	if r.Header.Get("X-Requested-With") != "" {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}
