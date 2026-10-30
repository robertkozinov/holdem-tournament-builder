package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"holdem-tournament-builder/internal/auth"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	redisstore "holdem-tournament-builder/internal/storage/redis"
	transport "holdem-tournament-builder/internal/transport/http"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticatedHTTP_Integration(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("integration test skipped: TEST_REDIS_URL is not set")
	}
	options, err := parseTestRedisURL(redisURL)
	require.NoError(t, err)
	bootstrapCtx, pool, cards := setupTournamentRepositoryTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	options.MaxRetries = -1
	options.Protocol = 2
	options.DialTimeout = time.Second
	options.ReadTimeout = time.Second
	options.WriteTimeout = time.Second
	options.ContextTimeoutEnabled = true
	redisClient := goredis.NewClient(options)
	t.Cleanup(func() { _ = redisClient.Close() })
	require.NoError(t, redisClient.Ping(bootstrapCtx).Err())
	users := NewUserRepository(pool)
	tokens := &auth.SessionTokenGenerator{}
	sessions := redisstore.NewSessionStore(redisClient)
	accounts := service.NewAuthService(users, sessions, &auth.PasswordHasher{}, tokens, 30*24*time.Hour)
	router := func() http.Handler {
		return transport.NewRouter(transport.NewTournamentHandler(service.NewTournamentService(cards)), transport.NewAuthHandler(accounts, transport.CookieOptions{Secure: true}, config.DefaultRequestSafety()), transport.NewPlayerHandler(service.NewPlayerService(NewPlayerRepository(pool))), transport.NewStatisticsHandler(service.NewStatisticsService(NewStatisticsRepository(pool))))
	}
	server := httptest.NewTLSServer(router())
	t.Cleanup(server.Close)
	newClient := func() *http.Client {
		jar, err := cookiejar.New(nil)
		require.NoError(t, err)
		return &http.Client{Transport: server.Client().Transport, Jar: jar, Timeout: 5 * time.Second}
	}
	owner, foreign, anonymous := newClient(), newClient(), newClient()
	request := func(client *http.Client, method, path string, body any, status int) (*http.Response, []byte) {
		t.Helper()
		var encoded []byte
		if body != nil {
			encoded, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(encoded))
		require.NoError(t, err)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, status, res.StatusCode, string(data))
		return res, data
	}
	password := "integration-only-pass!"
	loginOwner, loginForeign := "owner_"+uuid.NewString()[:8], "guest_"+uuid.NewString()[:8]
	credentials := func(login string) map[string]string { return map[string]string{"login": login, "password": password} }
	decodeUser := func(data []byte, expected string) uuid.UUID {
		t.Helper()
		var user struct {
			ID    uuid.UUID `json:"id"`
			Login string    `json:"login"`
		}
		require.NoError(t, json.Unmarshal(data, &user))
		assert.Equal(t, expected, user.Login)
		assert.NotEqual(t, uuid.Nil, user.ID)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &fields))
		assert.Len(t, fields, 2)
		assert.NotContains(t, string(data), password)
		assert.NotContains(t, string(data), "argon2id")
		return user.ID
	}
	registered, data := request(owner, http.MethodPost, "/auth/register", credentials("  "+strings.ToUpper(loginOwner)+"  "), http.StatusCreated)
	ownerID := decodeUser(data, loginOwner)
	cleanupAccount := func(id uuid.UUID) {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := pool.Exec(cleanup, "DELETE FROM tournaments WHERE user_id = $1", id)
			if err != nil {
				t.Error("clean up test account tournaments failed")
				return
			}
			if _, err := pool.Exec(cleanup, "DELETE FROM players WHERE user_id = $1", id); err != nil {
				t.Error("clean up test account players failed")
				return
			}
			if _, err := pool.Exec(cleanup, "DELETE FROM users WHERE id = $1", id); err != nil {
				t.Error("clean up test account failed")
			}
		})
	}
	cleanupAccount(ownerID)
	assert.Empty(t, registered.Cookies())
	_, data = request(foreign, http.MethodPost, "/auth/register", credentials(loginForeign), http.StatusCreated)
	foreignID := decodeUser(data, loginForeign)
	cleanupAccount(foreignID)
	assert.NotEqual(t, ownerID, foreignID)
	request(anonymous, http.MethodPost, "/auth/register", credentials(loginOwner), http.StatusConflict)
	_, missing := request(anonymous, http.MethodPost, "/auth/login", credentials("unknown_"+uuid.NewString()[:8]), http.StatusUnauthorized)
	_, wrong := request(anonymous, http.MethodPost, "/auth/login", map[string]string{"login": loginOwner, "password": "incorrect-password"}, http.StatusUnauthorized)
	assert.Equal(t, string(missing), string(wrong))
	keys := make([]string, 0, 3)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if len(keys) > 0 {
			_ = redisClient.Del(cleanup, keys...).Err()
		}
	})
	login := func(client *http.Client, name string) string {
		t.Helper()
		res, data := request(client, http.MethodPost, "/auth/login", credentials(name), http.StatusOK)
		decodeUser(data, name)
		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		assert.Equal(t, "__Host-holdem_session", cookie.Name)
		assert.True(t, cookie.Secure)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain)
		assert.Positive(t, cookie.MaxAge)
		assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
		hash, err := tokens.Hash(cookie.Value)
		require.NoError(t, err)
		key := "session:" + hash
		keys = append(keys, key)
		stored, err := redisClient.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.NotContains(t, stored, cookie.Value)
		return key
	}
	ownerKey, foreignKey := login(owner, loginOwner), login(foreign, loginForeign)
	_, data = request(owner, http.MethodGet, "/auth/me", nil, http.StatusOK)
	assert.Equal(t, ownerID, decodeUser(data, loginOwner))
	create := transport.CreateTournamentRequest{
		TournamentSettingsRequest: transport.TournamentSettingsRequest{
			Name: "Auth integration", Date: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
			Players: []transport.PlayerRequest{{Name: "A"}, {Name: "B"}}, SettlementMode: "bank",
			ChipSet: []transport.ChipRequest{{Value: 25, Count: 80}, {Value: 100, Count: 80}},
			Style:   "standard", DurationMinutes: 120, LevelDurationMinutes: 20,
		}, BuyInAmount: 1000,
	}
	_, data = request(owner, http.MethodPost, "/tournaments", create, http.StatusCreated)
	var created transport.CreateTournamentResponse
	require.NoError(t, json.Unmarshal(data, &created))
	path := "/tournaments/" + created.ID
	_, data = request(owner, http.MethodGet, path, nil, http.StatusOK)
	var card transport.TournamentResponse
	require.NoError(t, json.Unmarshal(data, &card))
	require.Len(t, card.Players, 2)
	patchPath := path + "/players/" + card.Players[0].ID
	save := transport.SaveTournamentRequest{TournamentSettingsRequest: create.TournamentSettingsRequest}
	save.Players = slices.Clone(create.Players)
	for i := range save.Players {
		save.Players[i].ID = uuid.MustParse(card.Players[i].ID)
	}
	patch := map[string]bool{"buy_in_paid": true}
	for _, operation := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/tournaments", create},
		{http.MethodGet, "/tournaments", nil},
		{http.MethodGet, path, nil},
		{http.MethodPut, path, save},
		{http.MethodPatch, patchPath, patch},
		{http.MethodDelete, path, nil},
	} {
		request(anonymous, operation.method, operation.path, operation.body, http.StatusUnauthorized)
	}
	for _, operation := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, path, nil}, {http.MethodPut, path, save},
		{http.MethodPatch, patchPath, patch}, {http.MethodDelete, path, nil},
	} {
		request(foreign, operation.method, operation.path, operation.body, http.StatusNotFound)
	}
	_, data = request(foreign, http.MethodGet, "/tournaments", nil, http.StatusOK)
	assert.JSONEq(t, "[]", string(data))
	request(foreign, http.MethodPost, "/tournaments", create, http.StatusCreated)
	_, data = request(owner, http.MethodGet, "/tournaments", nil, http.StatusOK)
	var listed []transport.TournamentResponse
	require.NoError(t, json.Unmarshal(data, &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	save.Name = "Saved by owner"
	request(owner, http.MethodPut, path, save, http.StatusOK)
	_, data = request(owner, http.MethodPatch, patchPath, patch, http.StatusOK)
	require.NoError(t, json.Unmarshal(data, &card))
	assert.True(t, card.Players[0].BuyInPaid)
	assert.Equal(t, save.Name, card.Name)
	firstTTL, err := redisClient.PTTL(ctx, ownerKey).Result()
	require.NoError(t, err)
	request(owner, http.MethodGet, path, nil, http.StatusOK)
	secondTTL, err := redisClient.PTTL(ctx, ownerKey).Result()
	require.NoError(t, err)
	assert.LessOrEqual(t, secondTTL, firstTTL)
	server.Close()
	restartedPool, err := NewPool(ctx, os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(restartedPool.Close)
	restartedRedis := goredis.NewClient(options)
	t.Cleanup(func() { _ = restartedRedis.Close() })
	require.NoError(t, restartedRedis.Ping(ctx).Err())
	restartedAccounts := service.NewAuthService(NewUserRepository(restartedPool), redisstore.NewSessionStore(restartedRedis), &auth.PasswordHasher{}, &auth.SessionTokenGenerator{}, 30*24*time.Hour)
	server = httptest.NewTLSServer(transport.NewRouter(
		transport.NewTournamentHandler(service.NewTournamentService(NewTournamentRepository(restartedPool))),
		transport.NewAuthHandler(restartedAccounts, transport.CookieOptions{Secure: true}, config.DefaultRequestSafety()),
		transport.NewPlayerHandler(service.NewPlayerService(NewPlayerRepository(restartedPool))),
		transport.NewStatisticsHandler(service.NewStatisticsService(NewStatisticsRepository(restartedPool))),
	))
	t.Cleanup(server.Close)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/auth/me", nil)
	require.NoError(t, err)
	res, err := owner.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())
	now := time.Now().UTC()
	expired, err := json.Marshal(domain.Session{UserID: foreignID, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)})
	require.NoError(t, err)
	require.NoError(t, redisClient.Set(ctx, foreignKey, expired, time.Hour).Err())
	request(foreign, http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	request(owner, http.MethodDelete, path, nil, http.StatusNoContent)
	request(owner, http.MethodGet, path, nil, http.StatusNotFound)
	oldCookies := owner.Jar.Cookies(mustParseTestURL(t, server.URL))
	require.Len(t, oldCookies, 1)
	loggedOut, _ := request(owner, http.MethodPost, "/auth/logout", nil, http.StatusNoContent)
	require.Len(t, loggedOut.Cookies(), 1)
	assert.Equal(t, -1, loggedOut.Cookies()[0].MaxAge)
	assert.Equal(t, "__Host-holdem_session", loggedOut.Cookies()[0].Name)
	assert.True(t, loggedOut.Cookies()[0].Secure)
	exists, err := redisClient.Exists(ctx, ownerKey).Result()
	require.NoError(t, err)
	assert.Zero(t, exists)
	request(owner, http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	revokedReq, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/auth/me", nil)
	require.NoError(t, err)
	revokedReq.AddCookie(oldCookies[0])
	res, err = anonymous.Do(revokedReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())
	request(owner, http.MethodPost, "/auth/logout", nil, http.StatusNoContent)
	unavailable := goredis.NewClient(options)
	require.NoError(t, unavailable.Close())
	unavailableService := service.NewAuthService(users, redisstore.NewSessionStore(unavailable), &auth.PasswordHasher{}, tokens, 30*24*time.Hour)
	down := httptest.NewTLSServer(transport.NewRouter(transport.NewTournamentHandler(service.NewTournamentService(cards)), transport.NewAuthHandler(unavailableService, transport.CookieOptions{Secure: true}, config.DefaultRequestSafety()), transport.NewPlayerHandler(service.NewPlayerService(NewPlayerRepository(pool))), transport.NewStatisticsHandler(service.NewStatisticsService(NewStatisticsRepository(pool)))))
	t.Cleanup(down.Close)
	downReq, err := http.NewRequestWithContext(ctx, http.MethodGet, down.URL+"/auth/me", nil)
	require.NoError(t, err)
	if cookies := foreign.Jar.Cookies(mustParseTestURL(t, server.URL)); len(cookies) > 0 {
		downReq.AddCookie(cookies[0])
	} else {
		t.Fatal("expected foreign session cookie")
	}
	res, err = anonymous.Do(downReq)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
}

func mustParseTestURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	require.NoError(t, err)
	return parsed
}
