package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type sessionClient interface {
	SetNX(context.Context, string, any, time.Duration) *goredis.BoolCmd
	Get(context.Context, string) *goredis.StringCmd
	Del(context.Context, ...string) *goredis.IntCmd
}

type SessionStore struct{ client sessionClient }

func NewSessionStore(client *goredis.Client) *SessionStore { return &SessionStore{client: client} }

func sessionKey(hash string) (string, error) {
	if len(hash) != 64 || hash != strings.ToLower(hash) {
		return "", fmt.Errorf("invalid session hash")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", fmt.Errorf("invalid session hash")
	}
	return "session:" + hash, nil
}

func validSession(session domain.Session) bool {
	return session.UserID != uuid.Nil && !session.CreatedAt.IsZero() && session.ExpiresAt.After(session.CreatedAt)
}

func (s *SessionStore) Create(ctx context.Context, hash string, session domain.Session) error {
	key, err := sessionKey(hash)
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrSessionUnavailable, err)
	}
	if !validSession(session) {
		return fmt.Errorf("%w: invalid session record", app.ErrSessionUnavailable)
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("%w: encode session: %w", app.ErrSessionUnavailable, err)
	}
	created, err := s.client.SetNX(ctx, key, data, session.ExpiresAt.Sub(session.CreatedAt)).Result()
	if err != nil {
		return fmt.Errorf("%w: create session: %w", app.ErrSessionUnavailable, err)
	}
	if !created {
		return fmt.Errorf("%w: session already exists", app.ErrSessionUnavailable)
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, hash string) (domain.Session, error) {
	key, err := sessionKey(hash)
	if err != nil {
		return domain.Session{}, fmt.Errorf("%w: %w", app.ErrSessionUnavailable, err)
	}
	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return domain.Session{}, app.ErrSessionNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("%w: get session: %w", app.ErrSessionUnavailable, err)
	}
	var session domain.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return domain.Session{}, fmt.Errorf("%w: decode session: %w", app.ErrSessionUnavailable, err)
	}
	if !validSession(session) {
		return domain.Session{}, fmt.Errorf("%w: invalid session record", app.ErrSessionUnavailable)
	}
	return session, nil
}

func (s *SessionStore) Delete(ctx context.Context, hash string) error {
	key, err := sessionKey(hash)
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrSessionUnavailable, err)
	}
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: delete session: %w", app.ErrSessionUnavailable, err)
	}
	return nil
}
