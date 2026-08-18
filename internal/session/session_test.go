package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestCreateGetDelete(t *testing.T) {
	ctx := context.Background()
	s := NewStore(testRedis(t), time.Hour)
	uid := uuid.New()
	sid, err := s.Create(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, sid)
	if err != nil || got != uid {
		t.Fatalf("get: %v %v", got, err)
	}
	if err := s.Delete(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, sid); err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestTouchUnknown(t *testing.T) {
	ctx := context.Background()
	s := NewStore(testRedis(t), time.Hour)
	if err := s.Touch(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestTouchExtendsTTL(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := NewStore(rdb, time.Hour)
	sid, err := s.Create(ctx, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(30 * time.Minute)
	if err := s.Touch(ctx, sid); err != nil {
		t.Fatal(err)
	}
	ttl := mr.TTL("session:" + sid)
	if ttl < 50*time.Minute {
		t.Fatalf("ttl after touch: %s", ttl)
	}
}

func TestDeleteUserSessions(t *testing.T) {
	ctx := context.Background()
	s := NewStore(testRedis(t), time.Hour)
	uid := uuid.New()
	sid1, _ := s.Create(ctx, uid)
	sid2, _ := s.Create(ctx, uid)
	if err := s.DeleteUserSessions(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, sid1); err != ErrNotFound {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, sid2); err != ErrNotFound {
		t.Fatal(err)
	}
}
