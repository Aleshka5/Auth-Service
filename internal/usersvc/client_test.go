package usersvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockUser struct {
	User
	roles []ServiceRole
}

type userSvcMock struct {
	mu          sync.Mutex
	bySub       map[string]*mockUser
	byID        map[string]*mockUser
	createCalls int
	getSubCalls int
	failCreate  int // HTTP status to return on POST, 0 = normal
	failGet     int
	failGetCode string
	seenAuth    []string
	seenCookie  []string
	seenXUser   []string
}

func newMock() *userSvcMock {
	return &userSvcMock{
		bySub: make(map[string]*mockUser),
		byID:  make(map[string]*mockUser),
	}
}

func (m *userSvcMock) put(u User, roles ...ServiceRole) {
	cp := &mockUser{User: u, roles: roles}
	m.bySub[u.GoogleSub] = cp
	m.byID[u.ID] = cp
}

func (m *userSvcMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.seenAuth = append(m.seenAuth, r.Header.Get("Authorization"))
	m.seenCookie = append(m.seenCookie, r.Header.Get("Cookie"))
	m.seenXUser = append(m.seenXUser, r.Header.Get("X-User-Id"))
	m.mu.Unlock()

	writeErr := func(status int, code, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": code, "message": msg},
		})
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/users":
		m.mu.Lock()
		m.createCalls++
		fail := m.failCreate
		m.mu.Unlock()
		if fail != 0 {
			code := CodeInternal
			if fail == http.StatusConflict {
				code = CodeUsernameExists
			}
			if fail == http.StatusForbidden {
				code = CodeForbidden
			}
			writeErr(fail, code, "forced")
			return
		}
		var body CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(http.StatusBadRequest, "INVALID_JSON", "invalid json")
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.bySub[body.GoogleSub]; ok {
			writeErr(http.StatusConflict, CodeGoogleSubExists, "A user with this google_sub already exists")
			return
		}
		for _, u := range m.byID {
			if strings.EqualFold(u.Username, body.Username) {
				writeErr(http.StatusConflict, CodeUsernameExists, "A user with this username already exists")
				return
			}
		}
		role := RoleStranger
		if len(m.byID) == 0 {
			role = RoleAdmin
		}
		u := User{
			ID:         "3fa85f64-5717-4562-b3fc-2c963f66afa6",
			GoogleSub:  body.GoogleSub,
			Email:      body.Email,
			Username:   body.Username,
			GlobalRole: role,
			CreatedAt:  time.Date(2026, 8, 29, 11, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 8, 29, 11, 0, 0, 0, time.UTC),
		}
		if len(m.byID) > 0 {
			u.ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
		}
		m.put(u)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/users/"+u.ID)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(u)
		return

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/by-google-sub/"):
		m.mu.Lock()
		m.getSubCalls++
		fail := m.failGet
		failCode := m.failGetCode
		m.mu.Unlock()
		if fail != 0 {
			if failCode == "" {
				failCode = CodeNotFound
			}
			writeErr(fail, failCode, "forced")
			return
		}
		sub := strings.TrimPrefix(r.URL.Path, "/users/by-google-sub/")
		m.mu.Lock()
		u, ok := m.bySub[sub]
		m.mu.Unlock()
		if !ok {
			writeErr(http.StatusNotFound, CodeNotFound, "User not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u.User)
		return

	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/roles") && strings.Count(r.URL.Path, "/") == 3:
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/users/"), "/roles")
		m.mu.Lock()
		u, ok := m.byID[id]
		m.mu.Unlock()
		if !ok {
			writeErr(http.StatusNotFound, CodeNotFound, "User not found")
			return
		}
		svcs := u.roles
		if svcs == nil {
			svcs = []ServiceRole{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RolesDocument{
			UserID:     u.ID,
			GlobalRole: u.GlobalRole,
			Services:   svcs,
		})
		return

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/"):
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		if strings.Contains(id, "/") {
			writeErr(http.StatusNotFound, CodeNotFound, "User not found")
			return
		}
		m.mu.Lock()
		u, ok := m.byID[id]
		m.mu.Unlock()
		if !ok {
			writeErr(http.StatusNotFound, CodeNotFound, "User not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u.User)
		return
	}

	writeErr(http.StatusNotFound, CodeNotFound, "User not found")
}

func (m *userSvcMock) assertNoJWT(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, a := range m.seenAuth {
		if a != "" {
			t.Fatalf("Authorization on request %d: %q", i, a)
		}
	}
	for i, c := range m.seenCookie {
		if c != "" {
			t.Fatalf("Cookie on request %d: %q", i, c)
		}
	}
	for i, x := range m.seenXUser {
		if x != "" {
			t.Fatalf("X-User-Id on request %d: %q", i, x)
		}
	}
}

func TestLookupOrCreateFirstLogin(t *testing.T) {
	m := newMock()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.LookupOrCreate(context.Background(), "110000000000000000001", "ada@example.com", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "110000000000000000001" || u.Username == "110000000000000000001" {
		t.Fatalf("id/username must not be google_sub: %+v", u)
	}
	if u.Username != "ada" || u.Email != "ada@example.com" || u.GlobalRole != RoleAdmin {
		t.Fatalf("%+v", u)
	}
	if m.createCalls != 1 || m.getSubCalls != 1 {
		t.Fatalf("calls create=%d get=%d", m.createCalls, m.getSubCalls)
	}
	m.assertNoJWT(t)
}

func TestLookupOrCreateExisting(t *testing.T) {
	m := newMock()
	m.put(User{
		ID: "3fa85f64-5717-4562-b3fc-2c963f66afa6", GoogleSub: "g1",
		Email: "ada@example.com", Username: "ada", GlobalRole: RoleAdmin,
	})
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.LookupOrCreate(context.Background(), "g1", "ada@example.com", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "3fa85f64-5717-4562-b3fc-2c963f66afa6" {
		t.Fatalf("%+v", u)
	}
	if m.createCalls != 0 {
		t.Fatalf("POST must not run on subsequent login, got %d", m.createCalls)
	}
}

func TestCreateUsernameCollisionRetry(t *testing.T) {
	m := newMock()
	m.put(User{
		ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", GoogleSub: "other",
		Email: "other@x.com", Username: "ada", GlobalRole: RoleStranger,
	})
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.LookupOrCreate(context.Background(), "g-new", "ada@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "ada_2" {
		t.Fatalf("want ada_2 got %q", u.Username)
	}
	if m.createCalls != 2 {
		t.Fatalf("create calls %d", m.createCalls)
	}
}

func TestCreateGoogleSubExistsThenGet(t *testing.T) {
	m := newMock()
	m.put(User{
		ID: "3fa85f64-5717-4562-b3fc-2c963f66afa6", GoogleSub: "g1",
		Email: "ada@example.com", Username: "ada", GlobalRole: RoleAdmin,
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/users/by-google-sub/") {
			m.mu.Lock()
			m.getSubCalls++
			n := m.getSubCalls
			m.mu.Unlock()
			if n == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"User not found"}}`)
				return
			}
		}
		m.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.LookupOrCreate(context.Background(), "g1", "ada@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "3fa85f64-5717-4562-b3fc-2c963f66afa6" {
		t.Fatalf("%+v", u)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	m := newMock()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetByID(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	if !IsNotFound(err) {
		t.Fatalf("err %v", err)
	}
}

func TestGetForbidden(t *testing.T) {
	m := newMock()
	m.failGet = http.StatusForbidden
	m.failGetCode = CodeForbidden
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetByGoogleSub(context.Background(), "g1")
	if !IsCode(err, CodeForbidden) {
		t.Fatalf("err %v", err)
	}
}

func TestCreateConflictForced(t *testing.T) {
	m := newMock()
	m.failCreate = http.StatusForbidden
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Create(context.Background(), "g", "a@b.c", "ada")
	if !IsCode(err, CodeForbidden) {
		t.Fatalf("err %v", err)
	}
}

func TestGetRoles(t *testing.T) {
	m := newMock()
	m.put(User{
		ID: "3fa85f64-5717-4562-b3fc-2c963f66afa6", GoogleSub: "g1",
		Email: "ada@example.com", Username: "ada", GlobalRole: RoleAdmin,
	}, ServiceRole{Service: "storage", Role: RoleFamily})
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := c.GetRoles(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6")
	if err != nil {
		t.Fatal(err)
	}
	if doc.GlobalRole != RoleAdmin || len(doc.Services) != 1 || doc.Services[0].Service != "storage" {
		t.Fatalf("%+v", doc)
	}
}

func TestClientNeverForwardsJWT(t *testing.T) {
	m := newMock()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/users/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig")
	req.Header.Set("Cookie", "auth_access=abc; auth_refresh=def")
	req.Header.Set("X-User-Id", "spoof")
	_ = c.do(req, http.StatusOK, nil)
	m.assertNoJWT(t)
}

func TestGetServiceRole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/3fa85f64-5717-4562-b3fc-2c963f66afa6/roles/storage" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("jwt must not be sent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"user_id":"3fa85f64-5717-4562-b3fc-2c963f66afa6","service":"storage","role":"FAMILY"}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	sr, err := c.GetServiceRole(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6", "storage")
	if err != nil {
		t.Fatal(err)
	}
	if sr.Service != "storage" || sr.Role != RoleFamily {
		t.Fatalf("%+v", sr)
	}
}

func TestGetServiceRoleNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"User not found"}}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetServiceRole(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6", "storage")
	if !IsNotFound(err) {
		t.Fatalf("got %v", err)
	}
}

func TestGetServiceRoleUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":{"code":"INTERNAL_ERROR","message":"down"}}`)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetServiceRole(context.Background(), "3fa85f64-5717-4562-b3fc-2c963f66afa6", "storage")
	if err == nil || IsNotFound(err) {
		t.Fatalf("got %v", err)
	}
	if statusOf(err) != http.StatusInternalServerError {
		t.Fatalf("status %v", err)
	}
}
