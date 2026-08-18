package users

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Memory is an in-process Repository for tests.
type Memory struct {
	mu         sync.Mutex
	adminEmail string
	byID       map[uuid.UUID]*User
	bySub      map[string]uuid.UUID
}

func NewMemory(adminEmail string) *Memory {
	return &Memory{
		adminEmail: strings.ToLower(strings.TrimSpace(adminEmail)),
		byID:       make(map[uuid.UUID]*User),
		bySub:      make(map[string]uuid.UUID),
	}
}

var _ Repository = (*Memory)(nil)

func (m *Memory) Put(u User) User {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	cp := u
	m.byID[u.ID] = &cp
	m.bySub[u.GoogleSub] = u.ID
	return cp
}

func (m *Memory) UpsertFromGoogle(_ context.Context, p GoogleProfile) (*User, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	email := strings.ToLower(strings.TrimSpace(p.Email))
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = email
	}

	if id, ok := m.bySub[p.Sub]; ok {
		u := cloneUser(m.byID[id])
		u.GoogleEmail = email
		u.Name = name
		u.UpdatedAt = time.Now().UTC()
		m.byID[id] = u
		return cloneUser(u), false, nil
	}

	role := RoleStranger
	if m.adminEmail != "" && email == m.adminEmail {
		role = RoleAdmin
	}
	now := time.Now().UTC()
	u := &User{
		ID:           uuid.New(),
		GoogleSub:    p.Sub,
		GoogleEmail:  email,
		Name:         name,
		Roles:        role,
		StorageRoles: ServiceRoleStranger,
		ResumeRoles:  ServiceRoleStranger,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.byID[u.ID] = u
	m.bySub[p.Sub] = u.ID
	return cloneUser(u), true, nil
}

func (m *Memory) GetByID(_ context.Context, id uuid.UUID) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneUser(u), nil
}

func (m *Memory) List(_ context.Context) ([]User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]User, 0, len(m.byID))
	for _, u := range m.byID {
		out = append(out, *cloneUser(u))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *Memory) UpdateRoles(_ context.Context, id uuid.UUID, patch RolePatch) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	demote := patch.Roles != nil && u.Roles == RoleAdmin && *patch.Roles != RoleAdmin
	if demote && RemovesLastAdmin(m.countActiveAdminsLocked(), *u, true, false) {
		return nil, ErrLastAdmin
	}
	if patch.Roles != nil {
		u.Roles = *patch.Roles
	}
	if patch.StorageRoles != nil {
		u.StorageRoles = *patch.StorageRoles
	}
	if patch.ResumeRoles != nil {
		u.ResumeRoles = *patch.ResumeRoles
	}
	u.UpdatedAt = time.Now().UTC()
	return cloneUser(u), nil
}

func (m *Memory) SetBlocked(_ context.Context, id uuid.UUID, blocked bool) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	if blocked && !u.Blocked && RemovesLastAdmin(m.countActiveAdminsLocked(), *u, false, true) {
		return nil, ErrLastAdmin
	}
	u.Blocked = blocked
	u.UpdatedAt = time.Now().UTC()
	return cloneUser(u), nil
}

func (m *Memory) TouchLastSeen(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	if u.LastSeenAt != nil && now.Sub(*u.LastSeenAt) < time.Minute {
		return nil
	}
	u.LastSeenAt = &now
	u.UpdatedAt = now
	return nil
}

func (m *Memory) countActiveAdminsLocked() int {
	n := 0
	for _, u := range m.byID {
		if u.Roles == RoleAdmin && !u.Blocked {
			n++
		}
	}
	return n
}

func cloneUser(u *User) *User {
	if u == nil {
		return nil
	}
	cp := *u
	if u.LastSeenAt != nil {
		t := *u.LastSeenAt
		cp.LastSeenAt = &t
	}
	return &cp
}
