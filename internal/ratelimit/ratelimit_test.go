package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAllow(t *testing.T) {
	mr := miniredis.RunT(t)
	l := New(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx, "oauth", "1.2.3.4", 3)
		if err != nil || !ok {
			t.Fatalf("iter %d: %v %v", i, ok, err)
		}
	}
	ok, err := l.Allow(ctx, "oauth", "1.2.3.4", 3)
	if err != nil || ok {
		t.Fatalf("expected deny, got %v %v", ok, err)
	}
	ok, err = l.Allow(ctx, "oauth", "9.9.9.9", 3)
	if err != nil || !ok {
		t.Fatalf("other ip should pass: %v %v", ok, err)
	}
}
