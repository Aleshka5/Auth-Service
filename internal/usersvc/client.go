package usersvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	CodeNotFound        = "NOT_FOUND"
	CodeUsernameExists  = "USERNAME_EXISTS"
	CodeGoogleSubExists = "GOOGLE_SUB_EXISTS"
	CodeForbidden       = "FORBIDDEN"
	CodeValidation      = "VALIDATION_ERROR"
	CodeInternal        = "INTERNAL_ERROR"
	maxUsernameAttempts = 8
	maxBody             = 1 << 20
)

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleFamily   Role = "FAMILY"
	RoleStranger Role = "STRANGER"
	RoleBlocked  Role = "BLOCKED"
)

type User struct {
	ID         string    `json:"id"`
	GoogleSub  string    `json:"google_sub"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	GlobalRole Role      `json:"global_role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ServiceRole struct {
	Service string `json:"service"`
	Role    Role   `json:"role"`
}

type RolesDocument struct {
	UserID     string        `json:"user_id"`
	GlobalRole Role          `json:"global_role"`
	Services   []ServiceRole `json:"services"`
}

type CreateUserRequest struct {
	GoogleSub string `json:"google_sub"`
	Email     string `json:"email"`
	Username  string `json:"username"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return "usersvc: error"
	}
	if e.Code != "" {
		return fmt.Sprintf("usersvc: %s (%d): %s", e.Code, e.Status, e.Message)
	}
	return fmt.Sprintf("usersvc: status %d", e.Status)
}

func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound) || (statusOf(err) == http.StatusNotFound)
}

func IsCode(err error, code string) bool {
	var api *APIError
	if !asAPIError(err, &api) {
		return false
	}
	return api.Code == code
}

func statusOf(err error) int {
	var api *APIError
	if !asAPIError(err, &api) {
		return 0
	}
	return api.Status
}

func asAPIError(err error, target **APIError) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*APIError)
	if !ok {
		return false
	}
	*target = e
	return true
}

type Client struct {
	base *url.URL
	http *http.Client
}

func New(baseURL string, hc *http.Client) (*Client, error) {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL is invalid")
	}
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{base: u, http: hc}, nil
}

func (c *Client) GetByID(ctx context.Context, id string) (*User, error) {
	return c.getUser(ctx, c.resolve("users", id))
}

func (c *Client) GetByGoogleSub(ctx context.Context, googleSub string) (*User, error) {
	return c.getUser(ctx, c.resolve("users", "by-google-sub", googleSub))
}

func (c *Client) Create(ctx context.Context, googleSub, email, username string) (*User, error) {
	body := CreateUserRequest{
		GoogleSub: googleSub,
		Email:     strings.ToLower(strings.TrimSpace(email)),
		Username:  username,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.resolve("users"), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var u User
	if err := c.do(req, http.StatusCreated, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) GetRoles(ctx context.Context, id string) (*RolesDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolve("users", id, "roles"), nil)
	if err != nil {
		return nil, err
	}
	var doc RolesDocument
	if err := c.do(req, http.StatusOK, &doc); err != nil {
		return nil, err
	}
	if doc.Services == nil {
		doc.Services = []ServiceRole{}
	}
	return &doc, nil
}

func (c *Client) GetServiceRole(ctx context.Context, id, service string) (*ServiceRole, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolve("users", id, "roles", service), nil)
	if err != nil {
		return nil, err
	}
	var sr ServiceRole
	if err := c.do(req, http.StatusOK, &sr); err != nil {
		return nil, err
	}
	return &sr, nil
}

// LookupOrCreate implements login orchestration: GET by google_sub, else POST /users
// with username collision retries. Never sends a JWT.
func (c *Client) LookupOrCreate(ctx context.Context, googleSub, email, name string) (*User, error) {
	u, err := c.GetByGoogleSub(ctx, googleSub)
	if err == nil {
		return u, nil
	}
	if !IsNotFound(err) {
		return nil, err
	}
	return c.createWithRetry(ctx, googleSub, email, name)
}

func (c *Client) createWithRetry(ctx context.Context, googleSub, email, name string) (*User, error) {
	base := DeriveUsername(email, name)
	var last error
	for attempt := 1; attempt <= maxUsernameAttempts; attempt++ {
		username := UsernameAttempt(base, attempt)
		u, err := c.Create(ctx, googleSub, email, username)
		if err == nil {
			return u, nil
		}
		last = err
		if IsCode(err, CodeGoogleSubExists) {
			return c.GetByGoogleSub(ctx, googleSub)
		}
		if IsCode(err, CodeUsernameExists) {
			continue
		}
		return nil, err
	}
	if last == nil {
		last = fmt.Errorf("username retries exhausted")
	}
	return nil, &APIError{Status: http.StatusServiceUnavailable, Code: CodeInternal, Message: last.Error()}
}

func (c *Client) getUser(ctx context.Context, rawURL string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	var u User
	if err := c.do(req, http.StatusOK, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) resolve(elem ...string) string {
	return c.base.JoinPath(elem...).String()
}

func (c *Client) do(req *http.Request, want int, dest any) error {
	stripAuthHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return err
	}
	if resp.StatusCode != want {
		return parseError(resp.StatusCode, body)
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode user-service: %w", err)
	}
	return nil
}

func stripAuthHeaders(req *http.Request) {
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	req.Header.Del("X-Auth-User-Id")
	req.Header.Del("X-Auth-Email")
	req.Header.Del("X-Auth-Name")
	req.Header.Del("X-Auth-Role")
	req.Header.Del("X-User-Id")
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseError(status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msg := strings.TrimSpace(env.Error.Message)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	code := env.Error.Code
	if code == "" && status == http.StatusNotFound {
		code = CodeNotFound
	}
	return &APIError{Status: status, Code: code, Message: msg}
}
