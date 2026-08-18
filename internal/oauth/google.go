package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	stateTTL     = 10 * time.Minute
	userInfoURL  = "https://openidconnect.googleapis.com/v1/userinfo"
	stateKeyPref = "oauth:state:"
)

var ErrInvalidState = errors.New("invalid oauth state")

type GoogleUser struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Exchanger is the OAuth surface used by HTTP handlers.
type Exchanger interface {
	AuthCodeURL(ctx context.Context, returnTo string) (string, error)
	ConsumeState(ctx context.Context, state string) (string, error)
	Exchange(ctx context.Context, code string) (*GoogleUser, error)
}

type Service struct {
	conf *oauth2.Config
	rdb  *redis.Client
}

var _ Exchanger = (*Service)(nil)

func New(cfg *config.Config, rdb *redis.Client) *Service {
	return &Service{
		conf: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		rdb: rdb,
	}
}

func (s *Service) AuthCodeURL(ctx context.Context, returnTo string) (string, error) {
	state, err := randomState()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, stateKeyPref+state, returnTo, stateTTL).Err(); err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}
	return s.conf.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

func (s *Service) ConsumeState(ctx context.Context, state string) (string, error) {
	if state == "" {
		return "", ErrInvalidState
	}
	key := stateKeyPref + state
	returnTo, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrInvalidState
	}
	if err != nil {
		return "", err
	}
	_ = s.rdb.Del(ctx, key).Err()
	return returnTo, nil
}

func (s *Service) Exchange(ctx context.Context, code string) (*GoogleUser, error) {
	tok, err := s.conf.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}
	client := s.conf.Client(ctx, tok)
	client.Timeout = 15 * time.Second
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseGoogleUser(resp.StatusCode, body)
}

func parseGoogleUser(status int, body []byte) (*GoogleUser, error) {
	if status != http.StatusOK {
		return nil, fmt.Errorf("userinfo: status %d", status)
	}
	var u GoogleUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.Name = strings.TrimSpace(u.Name)
	if u.Sub == "" || u.Email == "" {
		return nil, errors.New("userinfo missing sub or email")
	}
	return &u, nil
}

func randomState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
