package token

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	CookieAccess  = "auth_access"
	CookieRefresh = "auth_refresh"

	TypAccess  = "access"
	TypRefresh = "refresh"

	MinSecretLen = 32
)

var (
	ErrInvalid = errors.New("invalid token")
	ErrExpired = errors.New("token expired")
	ErrType    = errors.New("unexpected token type")
)

// AccessClaims is the Auth access JWT. sub is the User-Service UUID.
type AccessClaims struct {
	jwt.RegisteredClaims
	Typ        string `json:"typ"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	GlobalRole string `json:"global_role"`
}

func (c AccessClaims) Sub() string { return c.Subject }
func (c AccessClaims) JTI() string { return c.ID }

func (c AccessClaims) IssuedAtTime() time.Time {
	if c.IssuedAt == nil {
		return time.Time{}
	}
	return c.IssuedAt.Time
}

func (c AccessClaims) ExpiresAtTime() time.Time {
	if c.ExpiresAt == nil {
		return time.Time{}
	}
	return c.ExpiresAt.Time
}

// RefreshClaims is the Auth refresh JWT. No roles — rotation re-reads User-Service.
type RefreshClaims struct {
	jwt.RegisteredClaims
	Typ string `json:"typ"`
}

func (c RefreshClaims) Sub() string { return c.Subject }
func (c RefreshClaims) JTI() string { return c.ID }

func (c RefreshClaims) IssuedAtTime() time.Time {
	if c.IssuedAt == nil {
		return time.Time{}
	}
	return c.IssuedAt.Time
}

func (c RefreshClaims) ExpiresAtTime() time.Time {
	if c.ExpiresAt == nil {
		return time.Time{}
	}
	return c.ExpiresAt.Time
}

type Config struct {
	Secret     []byte
	Issuer     string
	Audiences  []string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type Service struct {
	secret     []byte
	issuer     string
	audiences  []string
	accessTTL  time.Duration
	refreshTTL time.Duration
	parser     *jwt.Parser
}

func New(cfg Config) (*Service, error) {
	if len(cfg.Secret) < MinSecretLen {
		return nil, fmt.Errorf("JWT secret must be at least %d bytes", MinSecretLen)
	}
	iss := strings.TrimSpace(cfg.Issuer)
	if iss == "" {
		return nil, fmt.Errorf("JWT issuer is required")
	}
	aud := uniqueAudiences(cfg.Audiences)
	if len(aud) == 0 {
		return nil, fmt.Errorf("JWT audience is required")
	}
	if cfg.AccessTTL <= 0 {
		return nil, fmt.Errorf("JWT access TTL must be positive")
	}
	if cfg.RefreshTTL <= 0 {
		return nil, fmt.Errorf("JWT refresh TTL must be positive")
	}
	return &Service{
		secret:     append([]byte(nil), cfg.Secret...),
		issuer:     iss,
		audiences:  aud,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(iss),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(5*time.Second),
		),
	}, nil
}

func (s *Service) AccessTTL() time.Duration  { return s.accessTTL }
func (s *Service) RefreshTTL() time.Duration { return s.refreshTTL }

func (s *Service) IssueAccess(sub, email, username, globalRole string) (string, AccessClaims, error) {
	now := time.Now()
	claims := AccessClaims{
		RegisteredClaims: s.registered(sub, now, s.accessTTL),
		Typ:              TypAccess,
		Email:            email,
		Username:         username,
		GlobalRole:       globalRole,
	}
	raw, err := s.sign(claims)
	return raw, claims, err
}

func (s *Service) IssueRefresh(sub string) (string, RefreshClaims, error) {
	now := time.Now()
	claims := RefreshClaims{
		RegisteredClaims: s.registered(sub, now, s.refreshTTL),
		Typ:              TypRefresh,
	}
	raw, err := s.sign(claims)
	return raw, claims, err
}

func (s *Service) ParseAccess(raw string) (*AccessClaims, error) {
	var claims AccessClaims
	if err := s.parse(raw, &claims); err != nil {
		return nil, err
	}
	if claims.Typ != TypAccess {
		return nil, ErrType
	}
	if claims.Subject == "" || claims.ID == "" {
		return nil, ErrInvalid
	}
	return &claims, nil
}

func (s *Service) ParseRefresh(raw string) (*RefreshClaims, error) {
	var claims RefreshClaims
	if err := s.parse(raw, &claims); err != nil {
		return nil, err
	}
	if claims.Typ != TypRefresh {
		return nil, ErrType
	}
	if claims.Subject == "" || claims.ID == "" {
		return nil, ErrInvalid
	}
	return &claims, nil
}

func (s *Service) registered(sub string, now time.Time, ttl time.Duration) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   sub,
		ID:        uuid.NewString(),
		Issuer:    s.issuer,
		Audience:  jwt.ClaimStrings(append([]string(nil), s.audiences...)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
}

func (s *Service) sign(claims jwt.Claims) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return raw, nil
}

func (s *Service) parse(raw string, claims jwt.Claims) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrInvalid
	}
	tok, err := s.parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalid
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return ErrExpired
		}
		return ErrInvalid
	}
	if !tok.Valid {
		return ErrInvalid
	}
	if !audienceAllowed(registeredAudience(claims), s.audiences) {
		return ErrInvalid
	}
	return nil
}

func registeredAudience(claims jwt.Claims) jwt.ClaimStrings {
	switch c := claims.(type) {
	case *AccessClaims:
		return c.Audience
	case *RefreshClaims:
		return c.Audience
	default:
		return nil
	}
}

func audienceAllowed(tokenAud, allowed []string) bool {
	if len(tokenAud) == 0 || len(allowed) == 0 {
		return false
	}
	ok := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		ok[strings.ToLower(a)] = struct{}{}
	}
	for _, a := range tokenAud {
		if _, found := ok[strings.ToLower(a)]; found {
			return true
		}
	}
	return false
}

func uniqueAudiences(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// RemainingTTL is the cache TTL for a rejected jti (at least 1s).
func RemainingTTL(exp time.Time) time.Duration {
	d := time.Until(exp)
	if d < time.Second {
		return time.Second
	}
	return d
}
