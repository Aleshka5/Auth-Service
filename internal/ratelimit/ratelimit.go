package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

func (l *Limiter) Allow(ctx context.Context, bucket, ip string, limit int) (bool, error) {
	window := time.Now().Unix() / 60
	key := fmt.Sprintf("rl:%s:%s:%d", bucket, ip, window)
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := l.rdb.Expire(ctx, key, 2*time.Minute).Err(); err != nil {
			return false, err
		}
	}
	return n <= int64(limit), nil
}
