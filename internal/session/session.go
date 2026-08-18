package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("session not found")

type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewStore(rdb *redis.Client, ttl time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl}
}

func (s *Store) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	sid, err := randomID()
	if err != nil {
		return "", err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sessionKey(sid), userID.String(), s.ttl)
	pipe.SAdd(ctx, userSessionsKey(userID), sid)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return sid, nil
}

func (s *Store) Get(ctx context.Context, sid string) (uuid.UUID, error) {
	if sid == "" {
		return uuid.Nil, ErrNotFound
	}
	val, err := s.rdb.Get(ctx, sessionKey(sid)).Result()
	if errors.Is(err, redis.Nil) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	return id, nil
}

func (s *Store) Touch(ctx context.Context, sid string) error {
	ok, err := s.rdb.Expire(ctx, sessionKey(sid), s.ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, sid string) error {
	uid, err := s.Get(ctx, sid)
	if errors.Is(err, ErrNotFound) {
		_ = s.rdb.Del(ctx, sessionKey(sid)).Err()
		return nil
	}
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, sessionKey(sid))
	pipe.SRem(ctx, userSessionsKey(uid), sid)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	ids, err := s.rdb.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, sessionKey(id))
	}
	pipe.Del(ctx, userSessionsKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}

func sessionKey(sid string) string {
	return "session:" + sid
}

func userSessionsKey(userID uuid.UUID) string {
	return "user:sessions:" + userID.String()
}

func randomID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
