package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testService(t *testing.T, ttlAccess, ttlRefresh time.Duration) *Service {
	t.Helper()
	s, err := New(Config{
		Secret:     bytes32("test-secret-key-32-bytes-long!!"),
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"filenkov.store", "localhost"},
		AccessTTL:  ttlAccess,
		RefreshTTL: ttlRefresh,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func bytes32(s string) []byte {
	b := []byte(s)
	if len(b) < MinSecretLen {
		padded := make([]byte, MinSecretLen)
		copy(padded, b)
		return padded
	}
	return b
}

func TestIssueParseAccess(t *testing.T) {
	s := testService(t, 15*time.Minute, 168*time.Hour)
	id := uuid.NewString()
	raw, claims, err := s.IssueAccess(id, "ada@example.com", "ada", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Typ != TypAccess || claims.Subject != id || claims.Email != "ada@example.com" || claims.Username != "ada" || claims.GlobalRole != "ADMIN" {
		t.Fatalf("%+v", claims)
	}
	if claims.ID == "" {
		t.Fatal("missing jti")
	}
	got, err := s.ParseAccess(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub() != id || got.JTI() != claims.ID || got.Email != "ada@example.com" || got.Username != "ada" || got.GlobalRole != "ADMIN" {
		t.Fatalf("%+v", got)
	}
	if strings.Contains(raw, "110000") {
		t.Fatal("google_sub must not appear in access token")
	}
}

func TestIssueParseRefresh(t *testing.T) {
	s := testService(t, 15*time.Minute, 168*time.Hour)
	id := uuid.NewString()
	raw, claims, err := s.IssueRefresh(id)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Typ != TypRefresh || claims.Subject != id || claims.ID == "" {
		t.Fatalf("%+v", claims)
	}
	got, err := s.ParseRefresh(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub() != id || got.JTI() != claims.ID {
		t.Fatalf("%+v", got)
	}
}

func TestParseAccessRejectsRefresh(t *testing.T) {
	s := testService(t, 15*time.Minute, 168*time.Hour)
	raw, _, err := s.IssueRefresh(uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ParseAccess(raw); err != ErrType {
		t.Fatalf("err %v", err)
	}
}

func TestParseRefreshRejectsAccess(t *testing.T) {
	s := testService(t, 15*time.Minute, 168*time.Hour)
	raw, _, err := s.IssueAccess(uuid.NewString(), "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ParseRefresh(raw); err != ErrType {
		t.Fatalf("err %v", err)
	}
}

func TestExpiredAccess(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	now := time.Now().Add(-time.Hour)
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    "https://filenkov.store",
			Audience:  jwt.ClaimStrings{"filenkov.store"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Typ:        TypAccess,
		Email:      "a@b.c",
		Username:   "a",
		GlobalRole: "STRANGER",
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(bytes32("test-secret-key-32-bytes-long!!"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ParseAccess(raw)
	if err != ErrExpired {
		t.Fatalf("err %v", err)
	}
}

func TestTamperedAccess(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	raw, _, err := s.IssueAccess(uuid.NewString(), "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	tampered := raw[:len(raw)-2] + "xx"
	if _, err := s.ParseAccess(tampered); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestWrongSecret(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	raw, _, err := s.IssueAccess(uuid.NewString(), "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(Config{
		Secret:     bytes32("other-secret-key-32-bytes-long!!"),
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"filenkov.store"},
		AccessTTL:  time.Hour,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ParseAccess(raw); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestWrongIssuer(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	raw, _, err := s.IssueAccess(uuid.NewString(), "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(Config{
		Secret:     bytes32("test-secret-key-32-bytes-long!!"),
		Issuer:     "https://evil.example",
		Audiences:  []string{"filenkov.store"},
		AccessTTL:  time.Hour,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ParseAccess(raw); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestWrongAudience(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	raw, _, err := s.IssueAccess(uuid.NewString(), "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(Config{
		Secret:     bytes32("test-secret-key-32-bytes-long!!"),
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"other.example"},
		AccessTTL:  time.Hour,
		RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ParseAccess(raw); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestEmptyToken(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	if _, err := s.ParseAccess(""); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
	if _, err := s.ParseRefresh("   "); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestNewRejectsShortSecret(t *testing.T) {
	_, err := New(Config{
		Secret:     []byte("short"),
		Issuer:     "https://filenkov.store",
		Audiences:  []string{"filenkov.store"},
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAlgNoneRejected(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			ID:        uuid.NewString(),
			Issuer:    "https://filenkov.store",
			Audience:  jwt.ClaimStrings{"filenkov.store"},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Typ:        TypAccess,
		Email:      "a@b.c",
		Username:   "a",
		GlobalRole: "STRANGER",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	raw, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ParseAccess(raw); err != ErrInvalid {
		t.Fatalf("err %v", err)
	}
}

func TestRemainingTTL(t *testing.T) {
	if d := RemainingTTL(time.Now().Add(-time.Hour)); d != time.Second {
		t.Fatalf("past %v", d)
	}
	if d := RemainingTTL(time.Now().Add(time.Hour)); d < 50*time.Minute {
		t.Fatalf("future %v", d)
	}
}

func TestFreshJTIEachIssue(t *testing.T) {
	s := testService(t, time.Hour, time.Hour)
	id := uuid.NewString()
	_, a, err := s.IssueAccess(id, "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := s.IssueAccess(id, "a@b.c", "a", "STRANGER")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("jti must be unique per access token")
	}
}
