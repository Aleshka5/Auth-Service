package rejected

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newCache(t *testing.T) *Cache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb)
}

func TestRejectJTI(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	jti := uuid.NewString()
	ok, err := c.IsRejected(ctx, jti, "", time.Now())
	if err != nil || ok {
		t.Fatalf("before: %v %v", ok, err)
	}
	if err := c.RejectJTI(ctx, jti, time.Minute); err != nil {
		t.Fatal(err)
	}
	ok, err = c.IsRejected(ctx, jti, "", time.Now())
	if err != nil || !ok {
		t.Fatalf("after: %v %v", ok, err)
	}
}

func TestRejectUserIAT(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	uid := uuid.NewString()
	before := time.Now().Add(-time.Minute)
	if err := c.RejectUser(ctx, uid, time.Hour); err != nil {
		t.Fatal(err)
	}
	ok, err := c.IsRejected(ctx, "", uid, before)
	if err != nil || !ok {
		t.Fatalf("old token must be rejected: %v %v", ok, err)
	}
	ok, err = c.IsRejected(ctx, "", uid, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("token issued after revocation must not match iat <= timestamp")
	}
}

func TestRejectEmptyNoop(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	if err := c.RejectJTI(ctx, "", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := c.RejectUser(ctx, "", time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestKeys(t *testing.T) {
	if JTIKey("abc") != "rejected:jti:abc" {
		t.Fatal(JTIKey("abc"))
	}
	if UserKey("u") != "rejected:user:u" {
		t.Fatal(UserKey("u"))
	}
}

func TestMinTTL(t *testing.T) {
	c := newCache(t)
	ctx := context.Background()
	jti := uuid.NewString()
	if err := c.RejectJTI(ctx, jti, 0); err != nil {
		t.Fatal(err)
	}
	ok, err := c.IsRejected(ctx, jti, "", time.Now())
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
}
