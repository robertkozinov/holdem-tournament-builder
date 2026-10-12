package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionClientStub struct {
	set func(context.Context, string, any, time.Duration) *goredis.BoolCmd
	get func(context.Context, string) *goredis.StringCmd
	del func(context.Context, ...string) *goredis.IntCmd
}

func (s *sessionClientStub) SetNX(ctx context.Context, key string, value any, ttl time.Duration) *goredis.BoolCmd {
	return s.set(ctx, key, value, ttl)
}
func (s *sessionClientStub) Get(ctx context.Context, key string) *goredis.StringCmd {
	return s.get(ctx, key)
}
func (s *sessionClientStub) Del(ctx context.Context, keys ...string) *goredis.IntCmd {
	return s.del(ctx, keys...)
}

func TestSessionStore(t *testing.T) {
	hash := strings.Repeat("a", 64)
	now := time.Now().UTC()
	session := domain.Session{UserID: uuid.New(), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	t.Run("creates expiring hashed-key session", func(t *testing.T) {
		client := &sessionClientStub{set: func(_ context.Context, key string, value any, ttl time.Duration) *goredis.BoolCmd {
			assert.Equal(t, "session:"+hash, key)
			assert.Equal(t, time.Hour, ttl)
			var got domain.Session
			require.NoError(t, json.Unmarshal(value.([]byte), &got))
			assert.Equal(t, session, got)
			return goredis.NewBoolResult(true, nil)
		}}
		require.NoError(t, (&SessionStore{client: client}).Create(context.Background(), hash, session))
	})
	t.Run("reads without extending TTL", func(t *testing.T) {
		data, err := json.Marshal(session)
		require.NoError(t, err)
		client := &sessionClientStub{get: func(_ context.Context, key string) *goredis.StringCmd {
			assert.Equal(t, "session:"+hash, key)
			return goredis.NewStringResult(string(data), nil)
		}}
		got, err := (&SessionStore{client: client}).Get(context.Background(), hash)
		require.NoError(t, err)
		assert.Equal(t, session, got)
	})
	t.Run("missing key is not found", func(t *testing.T) {
		client := &sessionClientStub{get: func(context.Context, string) *goredis.StringCmd { return goredis.NewStringResult("", goredis.Nil) }}
		got, err := (&SessionStore{client: client}).Get(context.Background(), hash)
		require.ErrorIs(t, err, app.ErrSessionNotFound)
		assert.Equal(t, domain.Session{}, got)
	})
	t.Run("removing absent session is idempotent", func(t *testing.T) {
		client := &sessionClientStub{del: func(_ context.Context, keys ...string) *goredis.IntCmd {
			assert.Equal(t, []string{"session:" + hash}, keys)
			return goredis.NewIntResult(0, nil)
		}}
		require.NoError(t, (&SessionStore{client: client}).Delete(context.Background(), hash))
	})
	t.Run("store failure preserves cause", func(t *testing.T) {
		cause := errors.New("connection unavailable")
		client := &sessionClientStub{get: func(context.Context, string) *goredis.StringCmd { return goredis.NewStringResult("", cause) }}
		_, err := (&SessionStore{client: client}).Get(context.Background(), hash)
		require.ErrorIs(t, err, app.ErrSessionUnavailable)
		assert.ErrorIs(t, err, cause)
	})
	t.Run("malformed stored record is not authentication", func(t *testing.T) {
		for _, data := range []string{"bad-json", `{}`, `{"user_id":"00000000-0000-0000-0000-000000000000"}`} {
			client := &sessionClientStub{get: func(context.Context, string) *goredis.StringCmd { return goredis.NewStringResult(data, nil) }}
			_, err := (&SessionStore{client: client}).Get(context.Background(), hash)
			require.ErrorIs(t, err, app.ErrSessionUnavailable)
		}
	})
	t.Run("malformed JSON preserves decoding error", func(t *testing.T) {
		client := &sessionClientStub{get: func(context.Context, string) *goredis.StringCmd {
			return goredis.NewStringResult("bad-json", nil)
		}}
		_, err := (&SessionStore{client: client}).Get(context.Background(), hash)
		require.ErrorIs(t, err, app.ErrSessionUnavailable)
		var syntaxErr *json.SyntaxError
		assert.ErrorAs(t, err, &syntaxErr)
	})
	t.Run("invalid hash rejected before Redis", func(t *testing.T) {
		for _, invalid := range []string{"", "raw-session-token", strings.Repeat("z", 64), strings.Repeat("a", 65)} {
			_, err := (&SessionStore{}).Get(context.Background(), invalid)
			require.Error(t, err)
		}
	})
	t.Run("rejects invalid session before Redis", func(t *testing.T) {
		for _, invalid := range []domain.Session{{}, {UserID: uuid.New(), CreatedAt: now, ExpiresAt: now}, {UserID: uuid.New(), ExpiresAt: now}} {
			require.ErrorIs(t, (&SessionStore{}).Create(context.Background(), hash, invalid), app.ErrSessionUnavailable)
		}
	})
	t.Run("write and delete failures preserve cause", func(t *testing.T) {
		cause := errors.New("write unavailable")
		client := &sessionClientStub{
			set: func(context.Context, string, any, time.Duration) *goredis.BoolCmd {
				return goredis.NewBoolResult(false, cause)
			},
			del: func(context.Context, ...string) *goredis.IntCmd { return goredis.NewIntResult(0, cause) },
		}
		for _, err := range []error{(&SessionStore{client: client}).Create(context.Background(), hash, session), (&SessionStore{client: client}).Delete(context.Background(), hash)} {
			require.ErrorIs(t, err, app.ErrSessionUnavailable)
			assert.ErrorIs(t, err, cause)
		}
	})
}
