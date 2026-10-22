package gateway

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	PolicyPublic  = "public"
	PolicySession = "session"
	PolicyAdmin   = "admin"
	PolicyReject  = "reject"

	upstreamUserService = "user_service"
	upstreamHermes      = "hermes"
)

const (
	roleAdmin   = "ADMIN"
	roleFamily  = "FAMILY"
	roleBlocked = "BLOCKED"
)

const (
	roleFieldStorage = "storage"
	roleFieldImage   = "image"
)

var knownRoleFields = map[string]struct{}{
	"global_role":    {},
	roleFieldStorage: {},
	"resume":         {},
	roleFieldImage:   {},
}

// File is the on-disk gateway YAML.
type File struct {
	Listen       string            `yaml:"listen"`
	PublicOrigin string            `yaml:"public_origin"`
	Timeouts     Timeouts          `yaml:"timeouts"`
	RateLimit    RateLimit         `yaml:"rate_limit"`
	Upstreams    map[string]string `yaml:"upstreams"`
	Hosts        map[string]*Host  `yaml:"hosts"`
}

type Timeouts struct {
	ReadHeader time.Duration
	Idle       time.Duration
	Proxy      time.Duration
}

func (t *Timeouts) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		ReadHeader string `yaml:"read_header"`
		Idle       string `yaml:"idle"`
		Proxy      string `yaml:"proxy"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	var err error
	if t.ReadHeader, err = parseOptDuration(raw.ReadHeader, 15*time.Second); err != nil {
		return fmt.Errorf("timeouts.read_header: %w", err)
	}
	if t.Idle, err = parseOptDuration(raw.Idle, 120*time.Second); err != nil {
		return fmt.Errorf("timeouts.idle: %w", err)
	}
	if t.Proxy, err = parseOptDuration(raw.Proxy, 60*time.Second); err != nil {
		return fmt.Errorf("timeouts.proxy: %w", err)
	}
	return nil
}

type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type HostTimeouts struct {
	Proxy time.Duration
}

func (t *HostTimeouts) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Proxy string `yaml:"proxy"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	rawProxy := strings.TrimSpace(raw.Proxy)
	if rawProxy == "" {
		return nil
	}
	d, err := time.ParseDuration(rawProxy)
	if err != nil {
		return fmt.Errorf("timeouts.proxy: %w", err)
	}
	if d <= 0 {
		return fmt.Errorf("timeouts.proxy: must be positive")
	}
	t.Proxy = d
	return nil
}

type Host struct {
	RoleField string        `yaml:"role_field"`
	Aliases   []string      `yaml:"aliases"`
	Routes    []Route       `yaml:"routes"`
	Timeouts  *HostTimeouts `yaml:"timeouts"`
}

type Route struct {
	Match           string   `yaml:"match"`
	Upstream        string   `yaml:"upstream"`
	Auth            string   `yaml:"auth"`
	StripPrefix     string   `yaml:"strip_prefix"`
	PreserveHost    *bool    `yaml:"preserve_host"`
	Methods         []string `yaml:"methods"`
	InjectAccessJWT *bool    `yaml:"inject_access_jwt"`
	Redirect        string   `yaml:"redirect"`
	// Headers are set on the upstream request, after the path rewrite. Values
	// support ${VAR} expansion from the gateway environment, so a shared
	// service token stays in .env and never in this file.
	Headers map[string]string `yaml:"headers"`
}

type compiledRoute struct {
	match           string
	upstreamName    string
	upstream        *url.URL
	auth            string
	stripPrefix     string
	preserveHost    bool
	methods         []string
	injectAccessJWT bool
	redirect        string
	headers         map[string]string
}

type compiledHost struct {
	name         string
	roleField    string
	proxyTimeout time.Duration // 0 = use file-level timeouts.proxy
	routes       []compiledRoute
}

// Router is a validated, host+path lookup table. First matching route wins.
type Router struct {
	file  *File
	hosts map[string]*compiledHost
}

func parseOptDuration(raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return d, nil
}

// LoadFile reads and validates a gateway YAML file.
func LoadFile(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse unmarshals and validates gateway YAML.
func Parse(data []byte) (*File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("gateway yaml: %w", err)
	}
	if err := f.applyDefaults(); err != nil {
		return nil, err
	}
	if _, err := f.Router(); err != nil {
		return nil, err
	}
	return &f, nil
}

func (f *File) applyDefaults() error {
	if strings.TrimSpace(f.Listen) == "" {
		f.Listen = ":8083"
	}
	if f.Timeouts.ReadHeader == 0 {
		f.Timeouts.ReadHeader = 15 * time.Second
	}
	if f.Timeouts.Idle == 0 {
		f.Timeouts.Idle = 120 * time.Second
	}
	if f.Timeouts.Proxy == 0 {
		f.Timeouts.Proxy = 60 * time.Second
	}
	if f.RateLimit.RequestsPerMinute < 0 {
		return fmt.Errorf("rate_limit.requests_per_minute must be >= 0")
	}
	if f.RateLimit.RequestsPerMinute == 0 {
		f.RateLimit.RequestsPerMinute = 180
	}
	if f.Upstreams == nil {
		f.Upstreams = map[string]string{}
	}
	return nil
}

// Router compiles host/path matchers. Safe to call more than once.
func (f *File) Router() (*Router, error) {
	if len(f.Hosts) == 0 {
		return nil, fmt.Errorf("hosts: at least one host is required")
	}
	r := &Router{
		file:  f,
		hosts: make(map[string]*compiledHost),
	}
	for name, h := range f.Hosts {
		name = normalizeHost(name)
		if name == "" {
			return nil, fmt.Errorf("hosts: empty hostname")
		}
		if h == nil {
			return nil, fmt.Errorf("hosts.%s: missing body", name)
		}
		ch, err := compileHost(name, h, f.Upstreams)
		if err != nil {
			return nil, err
		}
		if _, exists := r.hosts[name]; exists {
			return nil, fmt.Errorf("hosts: duplicate hostname %q", name)
		}
		r.hosts[name] = ch
		for _, alias := range h.Aliases {
			alias = normalizeHost(alias)
			if alias == "" {
				continue
			}
			if _, exists := r.hosts[alias]; exists {
				return nil, fmt.Errorf("hosts.%s: alias %q collides with another host", name, alias)
			}
			r.hosts[alias] = ch
		}
	}
	return r, nil
}

func compileHost(name string, h *Host, upstreams map[string]string) (*compiledHost, error) {
	roleField := strings.TrimSpace(h.RoleField)
	if roleField == "" {
		roleField = "global_role"
	}
	if _, ok := knownRoleFields[roleField]; !ok {
		return nil, fmt.Errorf("hosts.%s: unknown role_field %q (want global_role, storage, resume, or image)", name, roleField)
	}
	if len(h.Routes) == 0 {
		return nil, fmt.Errorf("hosts.%s: at least one route is required", name)
	}
	ch := &compiledHost{name: name, roleField: roleField, routes: make([]compiledRoute, 0, len(h.Routes))}
	if h.Timeouts != nil && h.Timeouts.Proxy > 0 {
		ch.proxyTimeout = h.Timeouts.Proxy
	}
	for i, rt := range h.Routes {
		cr, err := compileRoute(name, i, rt, upstreams)
		if err != nil {
			return nil, err
		}
		ch.routes = append(ch.routes, cr)
	}
	return ch, nil
}

func compileRoute(host string, i int, rt Route, upstreams map[string]string) (compiledRoute, error) {
	prefix := fmt.Sprintf("hosts.%s.routes[%d]", host, i)
	match := strings.TrimSpace(rt.Match)
	if match == "" {
		return compiledRoute{}, fmt.Errorf("%s: match is required", prefix)
	}
	if !strings.HasPrefix(match, "/") {
		return compiledRoute{}, fmt.Errorf("%s: match %q must start with /", prefix, match)
	}
	auth := strings.ToLower(strings.TrimSpace(rt.Auth))
	switch auth {
	case PolicyPublic, PolicySession, PolicyAdmin, PolicyReject:
	case "":
		return compiledRoute{}, fmt.Errorf("%s: auth is required (public, session, admin, or reject)", prefix)
	default:
		return compiledRoute{}, fmt.Errorf("%s: unknown auth %q", prefix, rt.Auth)
	}
	methods, err := normalizeMethods(rt.Methods)
	if err != nil {
		return compiledRoute{}, fmt.Errorf("%s: %w", prefix, err)
	}
	if auth == PolicyReject {
		return compiledRoute{
			match:   match,
			auth:    auth,
			methods: methods,
		}, nil
	}
	redirect := strings.TrimSpace(rt.Redirect)
	if redirect != "" {
		if !strings.HasPrefix(redirect, "/") {
			return compiledRoute{}, fmt.Errorf("%s: redirect must start with /", prefix)
		}
		if strings.Contains(redirect, "://") {
			return compiledRoute{}, fmt.Errorf("%s: redirect must be a same-origin path", prefix)
		}
		return compiledRoute{
			match:    match,
			auth:     auth,
			methods:  methods,
			redirect: redirect,
		}, nil
	}
	upName := strings.TrimSpace(rt.Upstream)
	if upName == "" {
		return compiledRoute{}, fmt.Errorf("%s: upstream is required", prefix)
	}
	raw, ok := upstreams[upName]
	if !ok {
		return compiledRoute{}, fmt.Errorf("%s: unknown upstream %q", prefix, upName)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return compiledRoute{}, fmt.Errorf("%s: upstream %q has invalid URL %q", prefix, upName, raw)
	}
	preserve := true
	if rt.PreserveHost != nil {
		preserve = *rt.PreserveHost
	}
	strip := strings.TrimSpace(rt.StripPrefix)
	if strip != "" && !strings.HasPrefix(strip, "/") {
		return compiledRoute{}, fmt.Errorf("%s: strip_prefix must start with /", prefix)
	}
	injectJWT := false
	if rt.InjectAccessJWT != nil {
		injectJWT = *rt.InjectAccessJWT
	}
	headers, err := compileHeaders(prefix, rt.Headers)
	if err != nil {
		return compiledRoute{}, err
	}
	return compiledRoute{
		match:           match,
		upstreamName:    upName,
		upstream:        u,
		auth:            auth,
		stripPrefix:     strings.TrimSuffix(strip, "/"),
		preserveHost:    preserve,
		methods:         methods,
		injectAccessJWT: injectJWT,
		headers:         headers,
	}, nil
}

// compileHeaders validates route headers and expands ${VAR} from the process
// environment. An unset variable is a configuration error at load time: a
// route that silently sends an empty service token would fail at request time
// with a confusing 401.
func compileHeaders(prefix string, raw map[string]string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raw))
	for name, value := range raw {
		key := http.CanonicalHeaderKey(strings.TrimSpace(name))
		if key == "" || strings.ContainsAny(key, " :	") {
			return nil, fmt.Errorf("%s: headers: invalid header name %q", prefix, name)
		}
		missing := ""
		expanded := os.Expand(value, func(varName string) string {
			v := os.Getenv(varName)
			if v == "" {
				missing = varName
			}
			return v
		})
		if missing != "" {
			return nil, fmt.Errorf("%s: headers.%s: environment variable %s is not set", prefix, key, missing)
		}
		if strings.ContainsAny(expanded, "\r\n") {
			return nil, fmt.Errorf("%s: headers.%s: value must be a single line", prefix, key)
		}
		out[key] = expanded
	}
	return out, nil
}

func normalizeMethods(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, m := range raw {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("methods: empty after trim")
	}
	return out, nil
}

// Hostnames returns configured host keys plus aliases (lowercased).
func (f *File) Hostnames() []string {
	if f == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(h string) {
		h = normalizeHost(h)
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	for name, h := range f.Hosts {
		add(name)
		if h == nil {
			continue
		}
		for _, a := range h.Aliases {
			add(a)
		}
	}
	return out
}

// AllowedHostsMap is the return_to allowlist shape used by oauth.ResolveReturnTo.
func (f *File) AllowedHostsMap() map[string][]string {
	m := make(map[string][]string)
	for _, h := range f.Hostnames() {
		m[h] = []string{}
	}
	return m
}

func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i]
	}
	return h
}
