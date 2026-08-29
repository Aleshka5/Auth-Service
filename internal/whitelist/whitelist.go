package whitelist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aleshka5/auth-service/internal/users"
	"github.com/redis/go-redis/v9"
)

var ErrUnknownHost = errors.New("unknown caller host")

type List struct {
	entries map[string][]string
}

func New(entries map[string][]string) *List {
	cp := make(map[string][]string, len(entries))
	for host, fields := range entries {
		cp[normalizeHost(host)] = append([]string{}, fields...)
	}
	return &List{entries: cp}
}

func (l *List) Persist(ctx context.Context, rdb *redis.Client) error {
	b, err := json.Marshal(l.entries)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, "whitelist", b, 0).Err()
}

func (l *List) HasHost(host string) bool {
	_, ok := l.entries[normalizeHost(host)]
	return ok
}

func (l *List) Project(host string, u *users.User) (map[string]string, error) {
	fields, ok := l.entries[normalizeHost(host)]
	if !ok {
		return nil, ErrUnknownHost
	}
	out := make(map[string]string, len(fields))
	for _, f := range fields {
		switch f {
		case "id":
			out[f] = u.ID.String()
		case "google_email":
			out[f] = u.GoogleEmail
		case "name":
			out[f] = u.Name
		case "roles":
			out[f] = string(u.Roles)
		case "storage_roles":
			out[f] = string(u.StorageRoles)
		case "resume_roles":
			out[f] = string(u.ResumeRoles)
		default:
			return nil, fmt.Errorf("unknown whitelist field %q", f)
		}
	}
	return out, nil
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
