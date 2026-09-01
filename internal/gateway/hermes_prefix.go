package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

const hermesDefaultLanding = "/sessions"
const hermesPublicPrefix = "/hermes"

// hermesDroppedAssetRedirect returns /hermes/assets/… when a Hermes page
// requests a Vite root-absolute /assets/* chunk. Redirecting (not rewriting)
// makes the module URL prefixed, so relative imports (xterm, etc.) stay under
// /hermes. Without this the hub SPA catch-all returns index.html and chat
// never opens a PTY.
func hermesDroppedAssetRedirect(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, hermesPublicPrefix+"/") {
		return ""
	}
	ref := strings.TrimSpace(r.Header.Get("Referer"))
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if u.Path != hermesPublicPrefix && !strings.HasPrefix(u.Path, hermesPublicPrefix+"/") {
		return ""
	}
	return hermesPublicPrefix + path
}

// hermesInternalPath is the path Hermes itself understands (no public mount).
func hermesInternalPath(path, prefix string) string {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if strings.Contains(path, "://") || strings.HasPrefix(path, "//") {
		return path
	}
	if prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
		path = stripPath(path, prefix)
	}
	if path == "" {
		return "/"
	}
	return path
}

func isHermesAuthPath(path string) bool {
	return path == "/login" || strings.HasPrefix(path, "/login/") ||
		strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/api/auth/")
}

// hermesBrowserPath is where the browser should go after a Hermes response.
// Localhost lands on /sessions after password login. "/" and /login must not
// be prefixed as /hermes/ or /hermes/login — that loops the login page.
func hermesBrowserPath(path, prefix string) string {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	internal := hermesInternalPath(path, prefix)
	if prefix == "" {
		if internal == "/" || internal == "" || isHermesAuthPath(internal) {
			return hermesDefaultLanding
		}
		return internal
	}
	if internal == "/" || internal == "" || isHermesAuthPath(internal) {
		return prefix + hermesDefaultLanding
	}
	return prefix + internal
}

func prefixPublicPath(path, prefix string) string {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	path = strings.TrimSpace(path)
	if prefix == "" {
		return path
	}
	if path == "" || path == "/" {
		return prefix + "/"
	}
	if strings.Contains(path, "://") || strings.HasPrefix(path, "//") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		return path
	}
	if path == prefix || strings.HasPrefix(path, prefix+"/") {
		return path
	}
	return prefix + path
}

// prefixPublicURL prefixes a Location path. The next= query stays
// Hermes-internal so the login form does not store /hermes/login.
func prefixPublicURL(raw, prefix string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.TrimSpace(prefix) == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme != "" || u.Host != "" {
		return raw
	}
	u.Path = prefixPublicPath(u.Path, prefix)
	q := u.Query()
	if next := q.Get("next"); next != "" {
		internal := hermesInternalPath(next, prefix)
		if isHermesAuthPath(internal) && q.Get("next") != hermesDefaultLanding {
			q.Set("next", hermesDefaultLanding)
			u.RawQuery = q.Encode()
		}
	}
	out := u.String()
	if out == "" {
		return prefixPublicPath("/", prefix)
	}
	return out
}

func rewriteHermesNextQuery(u *url.URL, prefix string) {
	if u == nil || strings.TrimSpace(prefix) == "" {
		return
	}
	q := u.Query()
	next := q.Get("next")
	if next == "" {
		return
	}
	// Keep /hermes/sessions (and other dashboard paths). Only rewrite
	// login/auth next values that would send the browser back to /login.
	internal := hermesInternalPath(next, prefix)
	if !isHermesAuthPath(internal) {
		return
	}
	q.Set("next", hermesDefaultLanding)
	u.RawQuery = q.Encode()
}

func applyHermesPublicPrefix(pr *httputil.ProxyRequest, t proxyTarget) {
	if pr == nil || pr.Out == nil || !hermesStripPrefix(t) {
		return
	}
	rewriteHermesNextQuery(pr.Out.URL, t.route.stripPrefix)
}

func hermesStripPrefix(t proxyTarget) bool {
	return t.route != nil && t.route.upstreamName == upstreamHermes && strings.TrimSpace(t.route.stripPrefix) != ""
}

func rewriteHermesPublicResponse(resp *http.Response, prefix string) error {
	if resp == nil || strings.TrimSpace(prefix) == "" {
		return nil
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		if rewritten := prefixPublicURL(loc, prefix); rewritten != loc {
			resp.Header.Set("Location", rewritten)
		}
	}
	if enc := strings.TrimSpace(resp.Header.Get("Content-Encoding")); enc != "" && enc != "identity" {
		return nil
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(ct, "application/json") {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return err
	}
	rewritten, ok := rewriteHermesNextJSON(body, prefix)
	if !ok {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	return nil
}

func rewriteHermesNextJSON(body []byte, prefix string) ([]byte, bool) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	next, ok := payload["next"].(string)
	if !ok {
		return nil, false
	}
	rewritten := hermesBrowserPath(next, prefix)
	if rewritten == next {
		return nil, false
	}
	payload["next"] = rewritten
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return out, true
}
