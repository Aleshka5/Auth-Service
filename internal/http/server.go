package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/rejected"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
)

type Server struct {
	cfg       *config.Config
	tokens    *token.Service
	rejected  *rejected.Cache
	users     *usersvc.Client
	oauth     oauth.Exchanger
	limiter   *ratelimit.Limiter
	pingRedis func() error
	sessions  *sid.Store
}

func New(
	cfg *config.Config,
	tokens *token.Service,
	rej *rejected.Cache,
	users *usersvc.Client,
	oauthSvc oauth.Exchanger,
	limiter *ratelimit.Limiter,
	pingRedis func() error,
	sessions *sid.Store,
) *Server {
	return &Server{
		cfg:       cfg,
		tokens:    tokens,
		rejected:  rej,
		users:     users,
		oauth:     oauthSvc,
		limiter:   limiter,
		pingRedis: pingRedis,
		sessions:  sessions,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	oauthLim := s.limit("oauth", 10)
	apiLim := s.limit("api", 60)

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.Handle("GET /oauth/google", oauthLim(http.HandlerFunc(s.handleOAuthStart)))
	mux.Handle("GET /oauth/google/callback", oauthLim(http.HandlerFunc(s.handleOAuthCallback)))
	mux.Handle("POST /api/logout", apiLim(http.HandlerFunc(s.handleLogout)))
	mux.Handle("POST /api/refresh", apiLim(http.HandlerFunc(s.handleRefresh)))
	mux.Handle("GET /api/me", apiLim(http.HandlerFunc(s.handleMe)))

	return mux
}

func (s *Server) limit(bucket string, n int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			ok, err := s.limiter.Allow(r.Context(), bucket, ip, n)
			if err != nil {
				slog.Error("rate_limit_error", "err", err, "ip", ip, "path", r.URL.Path)
				writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
				return
			}
			if !ok {
				slog.Warn("rate_limit", "ip", ip, "path", r.URL.Path, "bucket", bucket)
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) setAuthCookies(w http.ResponseWriter, access, refresh string) {
	http.SetCookie(w, s.cookie(token.CookieAccess, access, int(s.tokens.AccessTTL().Seconds())))
	http.SetCookie(w, s.cookie(token.CookieRefresh, refresh, int(s.tokens.RefreshTTL().Seconds())))
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, s.cookie(token.CookieAccess, "", -1))
	http.SetCookie(w, s.cookie(token.CookieRefresh, "", -1))
	http.SetCookie(w, s.cookie(sid.CookieName, "", -1))
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, s.cookie(sid.CookieName, value, int(s.tokens.RefreshTTL().Seconds())))
}

func (s *Server) cookie(name, value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if s.cfg.CookieDomain != "" {
		c.Domain = s.cfg.CookieDomain
	}
	return c
}

func (s *Server) parseAccess(r *http.Request) (*token.AccessClaims, error) {
	c, err := r.Cookie(token.CookieAccess)
	if err != nil || c.Value == "" {
		return nil, token.ErrInvalid
	}
	claims, err := s.tokens.ParseAccess(c.Value)
	if err != nil {
		return nil, err
	}
	if s.rejected != nil {
		ok, err := s.rejected.IsRejected(r.Context(), claims.JTI(), claims.Sub(), claims.IssuedAtTime())
		if err != nil {
			return nil, err
		}
		if ok {
			return nil, token.ErrInvalid
		}
	}
	return claims, nil
}

func (s *Server) parseRefresh(r *http.Request) (*token.RefreshClaims, error) {
	c, err := r.Cookie(token.CookieRefresh)
	if err != nil || c.Value == "" {
		return nil, token.ErrInvalid
	}
	claims, err := s.tokens.ParseRefresh(c.Value)
	if err != nil {
		return nil, err
	}
	if s.rejected != nil {
		ok, err := s.rejected.IsRejected(r.Context(), claims.JTI(), claims.Sub(), claims.IssuedAtTime())
		if err != nil {
			return nil, err
		}
		if ok {
			return nil, token.ErrInvalid
		}
	}
	return claims, nil
}

type meJSON struct {
	ID         string                `json:"id"`
	GoogleSub  string                `json:"google_sub"`
	Email      string                `json:"email"`
	Username   string                `json:"username"`
	GlobalRole string                `json:"global_role"`
	Services   []usersvc.ServiceRole `json:"services"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isUnauthorized(err error) bool {
	return errors.Is(err, token.ErrInvalid) || errors.Is(err, token.ErrExpired) || errors.Is(err, token.ErrType)
}
