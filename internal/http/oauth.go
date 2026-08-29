package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
)

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	returnTo, err := oauth.ResolveReturnTo(r.URL.Query().Get("return_to"), s.cfg.PublicOrigin, s.cfg.ReturnToAllowlist(), s.cfg.DevHTTP)
	if err != nil {
		slog.Warn("return_to_reject", "return_to", r.URL.Query().Get("return_to"), "err", err.Error())
		writeError(w, http.StatusBadRequest, "invalid return_to")
		return
	}
	url, err := s.oauth.AuthCodeURL(r.Context(), returnTo)
	if err != nil {
		slog.Error("oauth_start", "err", err)
		writeError(w, http.StatusInternalServerError, "oauth start failed")
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if msg := r.URL.Query().Get("error"); msg != "" {
		slog.Warn("oauth_provider_error", "error", msg)
		writeError(w, http.StatusBadRequest, "oauth error")
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}
	returnTo, err := s.oauth.ConsumeState(r.Context(), state)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	gu, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth_exchange", "err", err)
		writeError(w, http.StatusBadGateway, "oauth exchange failed")
		return
	}
	u, err := s.users.LookupOrCreate(r.Context(), gu.Sub, gu.Email, gu.Name)
	if err != nil {
		slog.Error("oauth_usersvc", "err", err)
		writeError(w, loginStatus(err), "login failed")
		return
	}
	if u.GlobalRole == usersvc.RoleBlocked {
		slog.Warn("login_blocked", "user_id", u.ID, "email", u.Email)
		writeError(w, http.StatusForbidden, "blocked")
		return
	}
	if err := s.issueSession(w, r, u, false); err != nil {
		slog.Error("jwt_issue", "err", err)
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	slog.Info("login", "user_id", u.ID, "email", u.Email, "username", u.Username)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var sub string
	if c, err := r.Cookie(token.CookieAccess); err == nil && c.Value != "" {
		if claims, err := s.tokens.ParseAccess(c.Value); err == nil {
			sub = claims.Sub()
			_ = s.rejected.RejectJTI(r.Context(), claims.JTI(), token.RemainingTTL(claims.ExpiresAtTime()))
		}
	}
	if c, err := r.Cookie(token.CookieRefresh); err == nil && c.Value != "" {
		if claims, err := s.tokens.ParseRefresh(c.Value); err == nil {
			if sub == "" {
				sub = claims.Sub()
			}
			_ = s.rejected.RejectJTI(r.Context(), claims.JTI(), token.RemainingTTL(claims.ExpiresAtTime()))
		}
	}
	if sub != "" {
		_ = s.rejected.RejectUser(r.Context(), sub, s.tokens.RefreshTTL())
		if s.sessions != nil {
			_ = s.sessions.DeleteByUser(r.Context(), sub)
		}
	}
	if c, err := r.Cookie(sid.CookieName); err == nil && c.Value != "" {
		if s.sessions != nil {
			_ = s.sessions.Delete(r.Context(), c.Value)
		}
	}
	s.clearAuthCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	claims, err := s.parseRefresh(r)
	if err != nil {
		s.dropCompanionSession(r, "")
		s.clearAuthCookies(w)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := s.users.GetByID(r.Context(), claims.Sub())
	if usersvc.IsNotFound(err) {
		_ = s.rejected.RejectJTI(r.Context(), claims.JTI(), token.RemainingTTL(claims.ExpiresAtTime()))
		s.dropCompanionSession(r, claims.Sub())
		s.clearAuthCookies(w)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		slog.Error("refresh_usersvc", "err", err)
		writeError(w, http.StatusBadGateway, "refresh failed")
		return
	}
	if u.GlobalRole == usersvc.RoleBlocked {
		_ = s.rejected.RejectJTI(r.Context(), claims.JTI(), token.RemainingTTL(claims.ExpiresAtTime()))
		s.dropCompanionSession(r, u.ID)
		s.clearAuthCookies(w)
		writeError(w, http.StatusForbidden, "blocked")
		return
	}
	_ = s.rejected.RejectJTI(r.Context(), claims.JTI(), token.RemainingTTL(claims.ExpiresAtTime()))
	if err := s.issueSession(w, r, u, true); err != nil {
		slog.Error("jwt_issue", "err", err)
		writeError(w, http.StatusInternalServerError, "refresh failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := s.parseAccess(r)
	if err != nil {
		if isUnauthorized(err) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		slog.Error("me_token", "err", err)
		writeError(w, http.StatusServiceUnavailable, "auth unavailable")
		return
	}
	u, err := s.users.GetByID(r.Context(), claims.Sub())
	if usersvc.IsNotFound(err) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		slog.Error("me_usersvc", "err", err)
		if usersvc.IsCode(err, usersvc.CodeForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusBadGateway, "user lookup failed")
		return
	}
	if u.GlobalRole == usersvc.RoleBlocked {
		writeError(w, http.StatusForbidden, "blocked")
		return
	}
	roles, err := s.users.GetRoles(r.Context(), u.ID)
	if usersvc.IsNotFound(err) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		slog.Error("me_roles", "err", err)
		writeError(w, http.StatusBadGateway, "user lookup failed")
		return
	}
	svcs := roles.Services
	if svcs == nil {
		svcs = []usersvc.ServiceRole{}
	}
	writeJSON(w, http.StatusOK, meJSON{
		ID:         u.ID,
		GoogleSub:  u.GoogleSub,
		Email:      u.Email,
		Username:   u.Username,
		GlobalRole: string(u.GlobalRole),
		Services:   svcs,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.pingRedis != nil {
		if err := s.pingRedis(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "redis"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, u *usersvc.User, keepSID bool) error {
	access, _, err := s.tokens.IssueAccess(u.ID, u.Email, u.Username, string(u.GlobalRole))
	if err != nil {
		return err
	}
	refresh, _, err := s.tokens.IssueRefresh(u.ID)
	if err != nil {
		return err
	}
	sessionID, err := s.resolveSID(r, u.ID, keepSID)
	if err != nil {
		return err
	}
	s.setAuthCookies(w, access, refresh)
	if sessionID != "" {
		s.setSessionCookie(w, sessionID)
	}
	return nil
}

func (s *Server) resolveSID(r *http.Request, userID string, keepSID bool) (string, error) {
	if s.sessions == nil {
		return "", nil
	}
	if keepSID {
		if c, err := r.Cookie(sid.CookieName); err == nil && c.Value != "" {
			uid, err := s.sessions.Get(r.Context(), c.Value)
			if err == nil && uid == userID {
				if err := s.sessions.Touch(r.Context(), c.Value, s.tokens.RefreshTTL()); err == nil {
					return c.Value, nil
				}
			}
		}
	}
	return s.sessions.Create(r.Context(), userID, s.tokens.RefreshTTL())
}

func (s *Server) dropCompanionSession(r *http.Request, userID string) {
	if s.sessions == nil {
		return
	}
	if userID != "" {
		_ = s.sessions.DeleteByUser(r.Context(), userID)
	}
	if c, err := r.Cookie(sid.CookieName); err == nil && c.Value != "" {
		_ = s.sessions.Delete(r.Context(), c.Value)
	}
}

func loginStatus(err error) int {
	api, ok := err.(*usersvc.APIError)
	if !ok {
		return http.StatusInternalServerError
	}
	if api.Status == http.StatusServiceUnavailable {
		return http.StatusServiceUnavailable
	}
	if api.Status >= 500 {
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}
