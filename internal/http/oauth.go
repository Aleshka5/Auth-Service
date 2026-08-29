package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/aleshka5/auth-service/internal/whitelist"
)

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	returnTo, err := oauth.ResolveReturnTo(r.URL.Query().Get("return_to"), s.cfg.PublicOrigin, s.cfg.Whitelist, s.cfg.DevHTTP)
	if err != nil {
		slog.Warn("whitelist_reject", "return_to", r.URL.Query().Get("return_to"), "err", err.Error())
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
	u, inserted, err := s.users.UpsertFromGoogle(r.Context(), users.GoogleProfile{
		Sub:   gu.Sub,
		Email: gu.Email,
		Name:  gu.Name,
	})
	if err != nil {
		slog.Error("oauth_upsert", "err", err)
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	host := returnHost(returnTo)
	if fields, projErr := whitelist.New(s.cfg.Whitelist).Project(host, u); projErr == nil && users.HasBlockedRole(fields) {
		slog.Warn("login_blocked", "user_id", u.ID.String(), "google_email", u.GoogleEmail, "host", host)
		writeError(w, http.StatusForbidden, "blocked")
		return
	}
	sid, err := s.sessions.Create(r.Context(), u.ID)
	if err != nil {
		slog.Error("session_create", "err", err)
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	s.setSessionCookie(w, sid)
	_ = s.users.TouchLastSeen(r.Context(), u.ID)
	slog.Info("login",
		"user_id", u.ID.String(),
		"google_email", u.GoogleEmail,
		"inserted", inserted,
	)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(s.cfg.CookieName)
	if err == nil && c.Value != "" {
		if delErr := s.sessions.Delete(r.Context(), c.Value); delErr != nil {
			slog.Error("logout", "err", delErr)
		}
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, _, ok := s.currentUser(w, r, true)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toUserJSON(u))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.pingDB != nil {
		if err := s.pingDB(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "postgres"})
			return
		}
	}
	if s.pingRedis != nil {
		if err := s.pingRedis(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "redis"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func returnHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
