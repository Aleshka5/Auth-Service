package sid

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb)
}

func TestCreateGetTouchDelete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	uid := uuid.NewString()

	id, err := s.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !validSID(id) {
		t.Fatalf("sid %q", id)
	}
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != sidBytes {
		t.Fatalf("hex %v %d", err, len(raw))
	}
	got, err := s.Get(ctx, id)
	if err != nil || got != uid {
		t.Fatalf("get %q %v", got, err)
	}

	if err := s.Touch(ctx, id, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}

func TestCreateReplacesPrevious(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	first, err := s.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected a new sid")
	}
	if _, err := s.Get(ctx, first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old sid should be gone: %v", err)
	}
	got, err := s.Get(ctx, second)
	if err != nil || got != uid {
		t.Fatalf("new sid %q %v", got, err)
	}
}

func TestDeleteByUser(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	uid := uuid.NewString()
	id, err := s.Create(ctx, uid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteByUser(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetRejectsInvalid(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get(context.Background(), "not-hex"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Get(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty: %v", err)
	}
}

func TestValidSID(t *testing.T) {
	if validSID("") || validSID("abc") || validSID("zz"+makeHex(t, 31)) {
		t.Fatal("expected invalid")
	}
	id, err := newSID()
	if err != nil || !validSID(id) {
		t.Fatalf("%q %v", id, err)
	}
}

func makeHex(t *testing.T, n int) string {
	t.Helper()
	raw := make([]byte, n)
	return hex.EncodeToString(raw)
}
