package sid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// CookieName is the companion opaque sid for product vaults. Not an edge credential.
	CookieName = "auth_session"

	keyPrefix  = "session:"
	userPrefix = "session:user:"
	sidBytes   = 32
)

var (
	ErrInvalid  = errors.New("invalid session id")
	ErrNotFound = errors.New("session not found")
)

// Store maps opaque auth_session values to User-Service UUIDs.
// Edge authorization stays on the access JWT; this is not a login session.
type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func sidKey(id string) string   { return keyPrefix + id }
func userKey(uid string) string { return userPrefix + uid }

func validSID(id string) bool {
	if len(id) != sidBytes*2 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func newSID() (string, error) {
	raw := make([]byte, sidBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("sid: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// Create mints a new sid for userID. Replaces any previous sid for that user.
func (s *Store) Create(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	if s == nil || s.rdb == nil {
		return "", fmt.Errorf("sid: store is required")
	}
	if userID == "" {
		return "", fmt.Errorf("sid: user id is required")
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	id, err := newSID()
	if err != nil {
		return "", err
	}
	if old, err := s.rdb.Get(ctx, userKey(userID)).Result(); err == nil && validSID(old) {
		_ = s.rdb.Del(ctx, sidKey(old)).Err()
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sidKey(id), userID, ttl)
	pipe.Set(ctx, userKey(userID), id, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("sid create: %w", err)
	}
	return id, nil
}

func (s *Store) Get(ctx context.Context, id string) (string, error) {
	if s == nil || s.rdb == nil {
		return "", fmt.Errorf("sid: store is required")
	}
	if !validSID(id) {
		return "", ErrInvalid
	}
	uid, err := s.rdb.Get(ctx, sidKey(id)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if uid == "" {
		return "", ErrNotFound
	}
	return uid, nil
}

func (s *Store) Touch(ctx context.Context, id string, ttl time.Duration) error {
	if ttl < time.Second {
		ttl = time.Second
	}
	uid, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Expire(ctx, sidKey(id), ttl)
	pipe.Expire(ctx, userKey(uid), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("sid touch: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if s == nil || s.rdb == nil || !validSID(id) {
		return nil
	}
	uid, err := s.rdb.Get(ctx, sidKey(id)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	keys := []string{sidKey(id)}
	if uid != "" {
		keys = append(keys, userKey(uid))
	}
	return s.rdb.Del(ctx, keys...).Err()
}

func (s *Store) DeleteByUser(ctx context.Context, userID string) error {
	if s == nil || s.rdb == nil || userID == "" {
		return nil
	}
	id, err := s.rdb.Get(ctx, userKey(userID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Delete(ctx, id)
}
