package gateway

import "strings"

func hostname(host string) string {
	return normalizeHost(host)
}

func matchPath(pattern, path string) bool {
	if path == "" {
		path = "/"
	}
	if pattern == "/*" || pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if prefix == "" {
			return true
		}
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return path == pattern
}

func matchMethods(allowed []string, method string) bool {
	if len(allowed) == 0 || method == "" {
		return true
	}
	method = strings.ToUpper(method)
	for _, m := range allowed {
		if m == method {
			return true
		}
	}
	return false
}

// Match returns the first matching host+route. ok is false for unknown host or no route.
func (r *Router) Match(host, path string) (*compiledHost, *compiledRoute, bool) {
	return r.MatchMethod(host, "", path)
}

// MatchMethod is Match with an HTTP method filter (empty method matches any).
func (r *Router) MatchMethod(host, method, path string) (*compiledHost, *compiledRoute, bool) {
	if r == nil {
		return nil, nil, false
	}
	ch, ok := r.hosts[hostname(host)]
	if !ok || ch == nil {
		return nil, nil, false
	}
	for i := range ch.routes {
		if !matchPath(ch.routes[i].match, path) {
			continue
		}
		if !matchMethods(ch.routes[i].methods, method) {
			continue
		}
		return ch, &ch.routes[i], true
	}
	return ch, nil, false
}

func stripPath(path, prefix string) string {
	if prefix == "" {
		return path
	}
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		out := path[len(prefix):]
		if out == "" {
			return "/"
		}
		return out
	}
	return path
}
