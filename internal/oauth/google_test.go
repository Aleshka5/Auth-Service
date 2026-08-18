package oauth

import (
	"context"
	"net/url"
	"testing"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestParseGoogleUser(t *testing.T) {
	u, err := parseGoogleUser(200, []byte(`{"sub":"1","email":" A@B.C ","name":" Ann "}`))
	if err != nil {
		t.Fatal(err)
	}
	if u.Sub != "1" || u.Email != "a@b.c" || u.Name != "Ann" {
		t.Fatalf("%+v", u)
	}
	if _, err := parseGoogleUser(401, []byte(`{}`)); err == nil {
		t.Fatal("expected status error")
	}
	if _, err := parseGoogleUser(200, []byte(`{"sub":"1"}`)); err == nil {
		t.Fatal("expected missing email")
	}
}

func TestAuthCodeURLAndConsumeState(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := New(&config.Config{
		GoogleClientID:     "id",
		GoogleClientSecret: "secret",
		GoogleRedirectURL:  "https://filenkov.store/oauth/google/callback",
	}, rdb)

	authURL, err := svc.AuthCodeURL(context.Background(), "https://storage.filenkov.store/")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("missing state in %q", authURL)
	}
	got, err := svc.ConsumeState(context.Background(), state)
	if err != nil || got != "https://storage.filenkov.store/" {
		t.Fatalf("consume: %q %v", got, err)
	}
	if _, err := svc.ConsumeState(context.Background(), state); err != ErrInvalidState {
		t.Fatalf("replay: %v", err)
	}
	if _, err := svc.ConsumeState(context.Background(), ""); err != ErrInvalidState {
		t.Fatalf("empty: %v", err)
	}
}
