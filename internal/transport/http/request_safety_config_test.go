package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/service"
)

func TestConfiguredRateLimitsThroughRouter(t *testing.T) {
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		rateKey  string
		burstKey string
		rate     string
		burst    int
		refill   time.Duration
		status   int
	}{
		{"general", http.MethodGet, "/auth/me", "HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE", "HTTP_RATE_LIMIT_BURST", "60", 2, time.Second, http.StatusOK},
		{"register", http.MethodPost, "/auth/register", "AUTH_REGISTER_REQUESTS_PER_MINUTE", "AUTH_REGISTER_BURST", "20", 1, 3 * time.Second, http.StatusCreated},
		{"login", http.MethodPost, "/auth/login", "AUTH_LOGIN_REQUESTS_PER_MINUTE", "AUTH_LOGIN_BURST", "15", 2, 4 * time.Second, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			safety := loadSafetyForHTTPTest(t, map[string]string{
				tc.rateKey:  tc.rate,
				tc.burstKey: strconv.Itoa(tc.burst),
			})
			now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
			h := NewAuthHandler(&fakeAuthService{}, CookieOptions{}, safety)
			h.now = func() time.Time { return now }
			router := NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{}), newStatisticsTestHandler())
			for range tc.burst {
				require.Equal(t, tc.status, safetyRequest(router, tc.method, tc.path, "192.0.2.1:10").Code)
			}
			rejected := safetyRequest(router, tc.method, tc.path, "192.0.2.1:20")
			require.Equal(t, http.StatusTooManyRequests, rejected.Code)
			assert.Equal(t, strconv.Itoa(int(tc.refill.Seconds())), rejected.Header().Get("Retry-After"))
			require.Equal(t, tc.status, safetyRequest(router, tc.method, tc.path, "192.0.2.2:10").Code)
			now = now.Add(tc.refill)
			require.Equal(t, tc.status, safetyRequest(router, tc.method, tc.path, "192.0.2.1:30").Code)
		})
	}
}

func TestConfiguredGeneralLimitIsSharedAcrossRoutes(t *testing.T) {
	safety := loadSafetyForHTTPTest(t, map[string]string{
		"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE": "1",
		"HTTP_RATE_LIMIT_BURST":               "1",
	})
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	h := NewAuthHandler(&fakeAuthService{}, CookieOptions{}, safety)
	h.now = func() time.Time { return now }
	router := NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{}), newStatisticsTestHandler())
	require.Equal(t, http.StatusOK, safetyRequest(router, http.MethodPost, "/auth/login", "192.0.2.1:10").Code)
	rejected := safetyRequest(router, http.MethodGet, "/auth/me", "192.0.2.1:20")
	assert.Equal(t, http.StatusTooManyRequests, rejected.Code)
	assert.Equal(t, "60", rejected.Header().Get("Retry-After"))
}

func TestConfiguredLimiterCleanupTiming(t *testing.T) {
	safety := loadSafetyForHTTPTest(t, map[string]string{
		"HTTP_RATE_LIMIT_MAX_CLIENTS":      "1",
		"HTTP_RATE_LIMIT_IDLE":             "2s",
		"HTTP_RATE_LIMIT_CLEANUP_INTERVAL": "5s",
	})
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	h := NewAuthHandler(&fakeAuthService{}, CookieOptions{}, safety)
	h.now = func() time.Time { return now }
	router := NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{}), newStatisticsTestHandler())
	require.Equal(t, http.StatusOK, safetyRequest(router, http.MethodGet, "/auth/me", "192.0.2.1:10").Code)
	now = now.Add(2 * time.Second)
	rejected := safetyRequest(router, http.MethodGet, "/auth/me", "192.0.2.2:10")
	require.Equal(t, http.StatusTooManyRequests, rejected.Code)
	assert.Equal(t, "3", rejected.Header().Get("Retry-After"))
	require.Len(t, h.safety.limiter.clients, 1)
	assert.Contains(t, h.safety.limiter.clients, "192.0.2.1")
	now = now.Add(3 * time.Second)
	require.Equal(t, http.StatusOK, safetyRequest(router, http.MethodGet, "/auth/me", "192.0.2.2:20").Code)
	require.Len(t, h.safety.limiter.clients, 1)
	assert.Contains(t, h.safety.limiter.clients, "192.0.2.2")
	assert.NotContains(t, h.safety.limiter.clients, "192.0.2.1")
}

func TestConfiguredAuthConcurrency(t *testing.T) {
	safety := loadSafetyForHTTPTest(t, map[string]string{"AUTH_MAX_CONCURRENT_REQUESTS": "3"})
	started := make(chan struct{}, 3)
	release, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	s := &fakeAuthService{login: func(_ context.Context, _, _ string, now time.Time) (service.AuthResult, error) {
		started <- struct{}{}
		<-release.Done()
		return service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: now.Add(24 * time.Hour)}, nil
	}}
	h := NewAuthHandler(s, CookieOptions{}, safety)
	require.Equal(t, 3, cap(h.safety.hashing))
	router := NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{}), newStatisticsTestHandler())
	results := make(chan *httptest.ResponseRecorder, 3)
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- safetyRequest(router, http.MethodPost, "/auth/login", fmt.Sprintf("192.0.2.%d:10", i+1))
		}()
	}
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("configured auth slot was not available")
		}
	}
	rejected := safetyRequest(router, http.MethodPost, "/auth/register", "192.0.2.50:10")
	assert.Equal(t, http.StatusServiceUnavailable, rejected.Code)
	assert.Empty(t, rejected.Result().Cookies())
	cancel()
	wg.Wait()
	for range 3 {
		assert.Equal(t, http.StatusOK, (<-results).Code)
	}
	assert.Empty(t, h.safety.hashing)
	assert.Equal(t, http.StatusOK, safetyRequest(router, http.MethodPost, "/auth/login", "192.0.2.100:10").Code)
}

func loadSafetyForHTTPTest(t *testing.T, overrides map[string]string) config.RequestSafety {
	t.Helper()
	env := map[string]string{
		"DATABASE_URL": "postgres://user:placeholder@localhost/holdem",
		"REDIS_URL":    "redis://127.0.0.1:6379/0",
	}
	for key, value := range overrides {
		env[key] = value
	}
	cfg, err := config.Load(func(key string) string { return env[key] })
	require.NoError(t, err)
	return cfg.RequestSafety
}
