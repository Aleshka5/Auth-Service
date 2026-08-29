package rejected

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	jtiPrefix  = "rejected:jti:"
	userPrefix = "rejected:user:"
)

type Cache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

func JTIKey(jti string) string {
	return jtiPrefix + jti
}

func UserKey(userID string) string {
	return userPrefix + userID
}

func (c *Cache) RejectJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if jti == "" {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	if err := c.rdb.Set(ctx, JTIKey(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("reject jti: %w", err)
	}
	return nil
}

func (c *Cache) RejectUser(ctx context.Context, userID string, ttl time.Duration) error {
	if userID == "" {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	val := strconv.FormatInt(time.Now().Unix(), 10)
	if err := c.rdb.Set(ctx, UserKey(userID), val, ttl).Err(); err != nil {
		return fmt.Errorf("reject user: %w", err)
	}
	return nil
}

// IsRejected reports whether the access/refresh token is revoked.
// A jti key is enough. A user key rejects tokens whose iat is at or before
// the stored revocation unix timestamp.
func (c *Cache) IsRejected(ctx context.Context, jti, userID string, iat time.Time) (bool, error) {
	if jti != "" {
		n, err := c.rdb.Exists(ctx, JTIKey(jti)).Result()
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	if userID == "" {
		return false, nil
	}
	val, err := c.rdb.Get(ctx, UserKey(userID)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sec, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return true, nil
	}
	revokedAt := time.Unix(sec, 0)
	if iat.IsZero() || !iat.After(revokedAt) {
		return true, nil
	}
	return false, nil
}
