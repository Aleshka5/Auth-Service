package httpapi

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/session"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/google/uuid"
)

type Server struct {
	cfg       *config.Config
	users     users.Repository
	sessions  *session.Store
	oauth     oauth.Exchanger
	limiter   *ratelimit.Limiter
	pingDB    func() error
	pingRedis func() error
}

func New(
	cfg *config.Config,
	usersStore users.Repository,
	sessions *session.Store,
	oauthSvc oauth.Exchanger,
	limiter *ratelimit.Limiter,
	pingDB func() error,
	pingRedis func() error,
) *Server {
	return &Server{
		cfg:       cfg,
		users:     usersStore,
		sessions:  sessions,
		oauth:     oauthSvc,
		limiter:   limiter,
		pingDB:    pingDB,
		pingRedis: pingRedis,
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
	mux.Handle("GET /api/me", apiLim(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/admin/users", apiLim(s.requireAdmin(s.handleAdminList)))
	mux.Handle("PATCH /api/admin/users/{id}", apiLim(s.requireAdmin(s.handleAdminPatch)))
	mux.Handle("POST /api/admin/users/{id}/block", apiLim(s.requireAdmin(s.handleAdminBlock)))

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

func (s *Server) requireAdmin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, ok := s.currentUser(w, r, true)
		if !ok {
			return
		}
		if u.Blocked || u.Roles != users.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request, touch bool) (*users.User, string, bool) {
	c, err := r.Cookie(s.cfg.CookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, "", false
	}
	uid, err := s.sessions.Get(r.Context(), c.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, "", false
	}
	u, err := s.users.GetByID(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, "", false
	}
	if u.Blocked {
		writeError(w, http.StatusForbidden, "blocked")
		return nil, "", false
	}
	if touch {
		_ = s.sessions.Touch(r.Context(), c.Value)
		_ = s.users.TouchLastSeen(r.Context(), u.ID)
		s.setSessionCookie(w, c.Value)
	}
	return u, c.Value, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, s.cookie(sid, int(s.cfg.SessionTTL.Seconds())))
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	c := s.cookie("", -1)
	http.SetCookie(w, c)
}

func (s *Server) cookie(value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     s.cfg.CookieName,
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

type userJSON struct {
	ID           string  `json:"id"`
	GoogleEmail  string  `json:"google_email"`
	Name         string  `json:"name"`
	Roles        string  `json:"roles"`
	StorageRoles string  `json:"storage_roles"`
	ResumeRoles  string  `json:"resume_roles"`
	Blocked      bool    `json:"blocked"`
	LastSeenAt   *string `json:"last_seen_at"`
}

func toUserJSON(u *users.User) userJSON {
	out := userJSON{
		ID:           u.ID.String(),
		GoogleEmail:  u.GoogleEmail,
		Name:         u.Name,
		Roles:        string(u.Roles),
		StorageRoles: string(u.StorageRoles),
		ResumeRoles:  string(u.ResumeRoles),
		Blocked:      u.Blocked,
	}
	if u.LastSeenAt != nil {
		s := u.LastSeenAt.UTC().Format(time.RFC3339)
		out.LastSeenAt = &s
	}
	return out
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

func parseID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}
