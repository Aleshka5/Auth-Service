package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/rejected"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
)

type ctxKey int

const proxyTargetKey ctxKey = 1

type proxyTarget struct {
	route *compiledRoute
	host  *compiledHost
	id    accessIdentity
}

// Server is the public HTTP edge: routing, JWT auth, reverse proxy.
type Server struct {
	cfg      *config.Config
	file     *File
	router   *Router
	tokens   *token.Service
	rej      *rejected.Cache
	limiter  *ratelimit.Limiter
	users    *usersvc.Client
	sessions *sid.Store
	proxy    *httputil.ReverseProxy
}

func New(cfg *config.Config, file *File, tokens *token.Service, rej *rejected.Cache, limiter *ratelimit.Limiter, users *usersvc.Client, sessions *sid.Store) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if file == nil {
		return nil, fmt.Errorf("gateway file is required")
	}
	router, err := file.Router()
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:      cfg,
		file:     file,
		router:   router,
		tokens:   tokens,
		rej:      rej,
		limiter:  limiter,
		users:    users,
		sessions: sessions,
	}
	s.proxy = s.newProxy()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	var h http.Handler = http.HandlerFunc(s.serve)
	h = s.withRateLimit(h)
	h = s.withAccessLog(h)
	h = s.withSecurityHeaders(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocketRequest(r) {
			applySecurityHeaders(w.Header(), !s.cfg.DevHTTP)
		}
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			s.handleHealthz(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if loc := hermesDroppedAssetRedirect(r); loc != "" {
		http.Redirect(w, r, loc, http.StatusFound)
		return
	}
	host, route, ok := s.router.MatchMethod(r.Host, r.Method, r.URL.Path)
	if !ok || route == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if route.auth == PolicyReject {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	id, err := s.lookupAccess(r)
	if err != nil {
		slog.Error("gateway_token", "err", err, "path", r.URL.Path)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth unavailable"})
		return
	}

	if !s.authorize(w, r, route.auth, id) {
		return
	}

	if route.redirect != "" {
		http.Redirect(w, r, route.redirect, http.StatusFound)
		return
	}

	if status, msg, ok := s.attachStorageRole(r.Context(), host, &id); !ok {
		writeJSON(w, status, map[string]string{"error": msg})
		return
	}

	stripIncomingAuth(r.Header)
	setForwarded(r, s.cfg.DevHTTP)

	ctx := context.WithValue(r.Context(), proxyTargetKey, proxyTarget{route: route, host: host, id: id})
	s.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.limiter == nil || s.file.RateLimit.RequestsPerMinute <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		ok, err := s.limiter.Allow(r.Context(), "gateway", ip, s.file.RateLimit.RequestsPerMinute)
		if err != nil {
			slog.Error("gateway_rate_limit_error", "err", err, "ip", ip, "path", r.URL.Path)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate limiter unavailable"})
			return
		}
		if !ok {
			slog.Warn("gateway_rate_limit", "ip", ip, "path", r.URL.Path)
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("access",
			"method", r.Method,
			"host", r.Host,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", clientIP(r),
		)
	})
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocketRequest(r) {
			applySecurityHeaders(w.Header(), !s.cfg.DevHTTP)
		}
		next.ServeHTTP(w, r)
	})
}

func newProxyTransport(responseHeaderTimeout time.Duration) *http.Transport {
	if responseHeaderTimeout <= 0 {
		responseHeaderTimeout = 60 * time.Second
	}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// HTTP/2 cannot Upgrade; Hermes PTY chat is a WebSocket.
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

type timeoutSwitch struct {
	byTimeout map[time.Duration]http.RoundTripper
	fallback  http.RoundTripper
}

func (t *timeoutSwitch) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := t.fallback
	if pt, ok := req.Context().Value(proxyTargetKey).(proxyTarget); ok && pt.host != nil && pt.host.proxyTimeout > 0 {
		if alt, ok := t.byTimeout[pt.host.proxyTimeout]; ok {
			tr = alt
		}
	}
	return tr.RoundTrip(req)
}

func (s *Server) newProxy() *httputil.ReverseProxy {
	defaultTO := s.file.Timeouts.Proxy
	if defaultTO <= 0 {
		defaultTO = 60 * time.Second
	}
	transports := map[time.Duration]http.RoundTripper{defaultTO: newProxyTransport(defaultTO)}
	if s.router != nil {
		for _, h := range s.router.hosts {
			if h == nil || h.proxyTimeout <= 0 {
				continue
			}
			if _, ok := transports[h.proxyTimeout]; ok {
				continue
			}
			transports[h.proxyTimeout] = newProxyTransport(h.proxyTimeout)
		}
	}
	p := &httputil.ReverseProxy{
		FlushInterval: -1,
		Rewrite: func(pr *httputil.ProxyRequest) {
			t, _ := pr.In.Context().Value(proxyTargetKey).(proxyTarget)
			if t.route == nil || t.route.upstream == nil {
				return
			}
			pr.SetURL(t.route.upstream)
			if t.route.preserveHost {
				pr.Out.Host = pr.In.Host
			}
			if t.route.stripPrefix != "" {
				pr.Out.URL.Path = stripPath(pr.In.URL.Path, t.route.stripPrefix)
				pr.Out.URL.RawPath = ""
			}
			s.applyProxyIdentity(pr, t)
			applyHermesPublicPrefix(pr, t)
		},
		Transport: &timeoutSwitch{byTimeout: transports, fallback: transports[defaultTO]},
		ModifyResponse: func(resp *http.Response) error {
			if resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
				return nil
			}
			applySecurityHeaders(resp.Header, !s.cfg.DevHTTP)
			s.maybeRejectTargetUser(resp)
			if resp != nil && resp.Request != nil {
				if t, ok := resp.Request.Context().Value(proxyTargetKey).(proxyTarget); ok && hermesStripPrefix(t) {
					return rewriteHermesPublicResponse(resp, t.route.stripPrefix)
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("gateway_proxy", "err", err, "host", r.Host, "path", r.URL.Path)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream unavailable"})
		},
	}
	return p
}

func forwardedProto(r *http.Request, devHTTP bool) string {
	if p := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); p != "" {
		if i := strings.IndexByte(p, ','); i >= 0 {
			p = strings.TrimSpace(p[:i])
		}
		return strings.ToLower(p)
	}
	if r.TLS != nil {
		return "https"
	}
	if devHTTP {
		return "http"
	}
	return "https"
}

func setForwarded(r *http.Request, devHTTP bool) {
	if r.Header.Get("X-Forwarded-Proto") == "" {
		r.Header.Set("X-Forwarded-Proto", forwardedProto(r, devHTTP))
	}
	if r.Header.Get("X-Forwarded-Host") == "" {
		r.Header.Set("X-Forwarded-Host", r.Host)
	}
}

func isWebSocketRequest(r *http.Request) bool {
	return r != nil && strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func applySecurityHeaders(h http.Header, hsts bool) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	if hsts {
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	if w.status == http.StatusOK {
		w.status = http.StatusSwitchingProtocols
	}
	return h.Hijack()
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
