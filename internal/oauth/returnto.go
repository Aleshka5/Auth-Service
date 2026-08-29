package oauth

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrInvalidReturnTo = errors.New("invalid return_to")
	ErrNotWhitelisted  = errors.New("return_to host is not whitelisted")
)

func ResolveReturnTo(raw, publicOrigin string, allowedHosts map[string][]string, devHTTP bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = publicOrigin
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return "", ErrInvalidReturnTo
	}
	if u.Fragment != "" {
		u.Fragment = ""
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", ErrInvalidReturnTo
	}

	switch u.Scheme {
	case "https":
		if _, ok := allowedHosts[host]; ok {
			return u.String(), nil
		}
		if devHTTP && isLoopback(host) {
			return u.String(), nil
		}
		return "", ErrNotWhitelisted
	case "http":
		if devHTTP && isLoopback(host) {
			return u.String(), nil
		}
		return "", ErrNotWhitelisted
	default:
		return "", ErrInvalidReturnTo
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
