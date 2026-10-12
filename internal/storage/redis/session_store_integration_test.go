package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSessionStoreTest(t *testing.T) (context.Context, *goredis.Client, *SessionStore) {
	t.Helper()
	endpoint := os.Getenv("TEST_REDIS_URL")
	if endpoint == "" {
		t.Skip("integration test skipped: TEST_REDIS_URL is not set")
	}
	opts, err := parseTestRedisURL(endpoint)
	require.NoError(t, err)
	opts.MaxRetries = -1
	opts.DialTimeout = time.Second
	opts.ReadTimeout = time.Second
	opts.WriteTimeout = time.Second
	client := goredis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, client.Ping(ctx).Err())
	return ctx, client, NewSessionStore(client)
}

func parseTestRedisURL(raw string) (*goredis.Options, error) {
	opts, err := goredis.ParseURL(raw)
	if err != nil {
		return nil, errors.New("invalid TEST_REDIS_URL")
	}
	host, _, err := net.SplitHostPort(opts.Addr)
	ip := net.ParseIP(host)
	if err != nil || !(host == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil, errors.New("only isolated local Redis is allowed in integration tests")
	}
	return opts, nil
}

func TestTestRedisURLDoesNotExposeCredentials(t *testing.T) {
	_, err := parseTestRedisURL("redis://user:test-secret@[invalid")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-secret")
	assert.NotContains(t, err.Error(), "://")
}

func TestSessionStore_Integration(t *testing.T) {
	ctx, client, store := setupSessionStoreTest(t)
	sum := sha256.Sum256([]byte(uuid.NewString()))
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	session := domain.Session{UserID: uuid.New(), CreatedAt: now, ExpiresAt: now.Add(2 * time.Second)}
	require.NoError(t, store.Create(ctx, hash, session))
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(cleanup, "session:"+hash).Err()
	})
	got, err := store.Get(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, session, got)
	require.ErrorIs(t, store.Create(ctx, hash, session), app.ErrSessionUnavailable)
	firstTTL, err := client.PTTL(ctx, "session:"+hash).Result()
	require.NoError(t, err)
	require.Positive(t, firstTTL)
	assert.LessOrEqual(t, firstTTL, 2*time.Second)
	time.Sleep(20 * time.Millisecond)
	_, err = store.Get(ctx, hash)
	require.NoError(t, err)
	secondTTL, err := client.PTTL(ctx, "session:"+hash).Result()
	require.NoError(t, err)
	assert.Less(t, secondTTL, firstTTL)
	require.Eventually(t, func() bool { _, err := store.Get(ctx, hash); return err == app.ErrSessionNotFound }, 3*time.Second, 20*time.Millisecond)
	_, err = store.Get(ctx, hash)
	require.ErrorIs(t, err, app.ErrSessionNotFound)
	require.NoError(t, store.Delete(ctx, hash))
	session.CreatedAt = time.Now().UTC()
	session.ExpiresAt = session.CreatedAt.Add(time.Hour)
	require.NoError(t, store.Create(ctx, hash, session))
	require.NoError(t, store.Delete(ctx, hash))
	_, err = store.Get(ctx, hash)
	require.ErrorIs(t, err, app.ErrSessionNotFound)
}

func TestSessionStore_UnreachableRedis(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sum := sha256.Sum256([]byte("unreachable-test"))
	_, err := NewSessionStore(client).Get(ctx, hex.EncodeToString(sum[:]))
	require.ErrorIs(t, err, app.ErrSessionUnavailable)
}
