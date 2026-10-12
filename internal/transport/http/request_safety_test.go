package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
)

func safetyRequest(router http.Handler, method, path, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(authBody))
	r.RemoteAddr = ip
	r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "test-session"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestRateLimitsBeforeServiceAndClockRefill(t *testing.T) {
	for _, tt := range []struct {
		path   string
		method string
		burst  int
		refill time.Duration
	}{
		{"/auth/register", http.MethodPost, 3, 12 * time.Second},
		{"/auth/login", http.MethodPost, 10, 6 * time.Second},
		{"/auth/me", http.MethodGet, 60, time.Second / 2},
		{"/tournaments", http.MethodGet, 60, time.Second / 2},
	} {
		t.Run(tt.path, func(t *testing.T) {
			var calls atomic.Int64
			s := &fakeAuthService{register: func(_ context.Context, _, _ string, now time.Time) (domain.User, error) {
				calls.Add(1)
				return domain.User{ID: testUserID, Login: "organizer", CreatedAt: now}, nil
			}, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
				calls.Add(1)
				return service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}, nil
			}, authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { calls.Add(1); return testUserID, nil }}
			h, router := authRouter(s, false)
			now := h.now()
			h.now = func() time.Time { return now }
			for range tt.burst {
				w := safetyRequest(router, tt.method, tt.path, "192.0.2.1:10")
				require.Less(t, w.Code, 400, w.Body.String())
			}
			before := calls.Load()
			w := safetyRequest(router, tt.method, tt.path, "192.0.2.1:20")
			assert.Equal(t, 429, w.Code)
			seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
			require.NoError(t, err)
			assert.Greater(t, seconds, 0)
			assert.Equal(t, before, calls.Load())
			now = now.Add(tt.refill)
			assert.Less(t, safetyRequest(router, tt.method, tt.path, "192.0.2.1:30").Code, 400)
		})
	}
}

func TestForwardedHeadersCannotBypassRateLimit(t *testing.T) {
	_, router := authRouter(&fakeAuthService{}, false)
	for i := range 11 {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(authBody))
		r.RemoteAddr = "[2001:db8::1]:" + strconv.Itoa(1000+i)
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", i))
		r.Header.Set("X-Real-IP", fmt.Sprintf("192.0.2.%d", i))
		r.Header.Set("Forwarded", fmt.Sprintf("for=192.0.2.%d", i))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if i < 10 {
			assert.Equal(t, 200, w.Code)
		} else {
			assert.Equal(t, 429, w.Code)
		}
	}
}

func TestLimiterBoundsAndIdleCleanup(t *testing.T) {
	h, router := authRouter(&fakeAuthService{}, false)
	require.Equal(t, 10000, h.safety.limiter.maxClients)
	h.safety.limiter.maxClients = 4
	now := h.now()
	h.now = func() time.Time { return now }
	for i := range h.safety.limiter.maxClients {
		w := safetyRequest(router, http.MethodGet, "/auth/me", fmt.Sprintf("10.0.%d.%d:10", i/256, i%256))
		require.Equal(t, 200, w.Code, "entry %d", i)
	}
	assert.Equal(t, 429, safetyRequest(router, http.MethodGet, "/auth/me", "198.51.100.1:10").Code)
	assert.Len(t, h.safety.limiter.clients, 4)
	assert.Equal(t, 200, safetyRequest(router, http.MethodGet, "/health", "198.51.100.1:10").Code)
	assert.Equal(t, 404, safetyRequest(router, http.MethodPost, "/tournaments/one/start", "198.51.100.1:10").Code)
	now = now.Add(11 * time.Minute)
	assert.Equal(t, 200, safetyRequest(router, http.MethodGet, "/auth/me", "198.51.100.1:10").Code)
	assert.Len(t, h.safety.limiter.clients, 1)
}

func TestLimiterDefaultCapacityWithoutHTTPLogging(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	limiter := newRequestSafety(func() time.Time { return now }, config.DefaultRequestSafety()).limiter
	for i := range 10000 {
		allowed, _ := limiter.allow(strconv.Itoa(i), generalRate)
		require.True(t, allowed, "client %d", i)
	}
	for i := range 100 {
		allowed, retry := limiter.allow("extra-"+strconv.Itoa(i), generalRate)
		assert.False(t, allowed)
		assert.Positive(t, retry)
	}
	assert.Len(t, limiter.clients, 10000)
}

func TestHashingConcurrencyIsSharedAndReleased(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	var calls atomic.Int64
	s := &fakeAuthService{register: func(context.Context, string, string, time.Time) (domain.User, error) {
		calls.Add(1)
		return domain.User{ID: testUserID, Login: "organizer"}, nil
	}, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
		calls.Add(1)
		started <- struct{}{}
		<-release
		return service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}, nil
	}}
	_, router := authRouter(s, false)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			safetyRequest(router, http.MethodPost, "/auth/login", fmt.Sprintf("192.0.2.%d:1", i))
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("hash request did not start")
		}
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- safetyRequest(router, http.MethodPost, "/auth/register", "192.0.2.50:1") }()
	select {
	case w := <-done:
		assert.Equal(t, 503, w.Code)
		assert.Empty(t, w.Result().Cookies())
	case <-time.After(time.Second):
		assert.Fail(t, "third hashing request was not rejected")
	}
	close(release)
	wg.Wait()
	assert.Equal(t, int64(2), calls.Load())
	assert.Equal(t, 200, safetyRequest(router, http.MethodPost, "/auth/login", "192.0.2.100:1").Code)
}

func TestHashingSlotsReleaseAfterCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	s := &fakeAuthService{login: func(ctx context.Context, _ string, _ string, _ time.Time) (service.AuthResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		return service.AuthResult{}, ctx.Err()
	}}
	_, router := authRouter(s, false)
	var wg sync.WaitGroup
	cancels := make([]context.CancelFunc, 2)
	for i := range 2 {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		defer cancel()
		r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(authBody)).WithContext(ctx)
		r.RemoteAddr = fmt.Sprintf("192.0.2.%d:1", i)
		wg.Add(1)
		go func() { defer wg.Done(); router.ServeHTTP(httptest.NewRecorder(), r) }()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("hash request did not start")
		}
	}
	assert.Equal(t, 503, safetyRequest(router, http.MethodPost, "/auth/register", "192.0.2.50:1").Code)
	for _, cancel := range cancels {
		cancel()
	}
	wg.Wait()
	assert.Equal(t, 201, safetyRequest(router, http.MethodPost, "/auth/register", "192.0.2.51:1").Code)
}

func TestCanceledRequestDoesNotReachService(t *testing.T) {
	called := false
	_, router := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called = true; return testUserID, nil }}, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil).WithContext(ctx)
	r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "test-session"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	assert.Equal(t, 503, w.Code)
	assert.False(t, called)
}

type cancelingBody struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (b *cancelingBody) Read(p []byte) (int, error) { b.cancel(); return b.reader.Read(p) }

func TestCancellationDuringDecodingAvoidsHashing(t *testing.T) {
	for _, path := range []string{"/auth/register", "/auth/login"} {
		t.Run(path, func(t *testing.T) {
			called := false
			h, router := authRouter(&fakeAuthService{register: func(context.Context, string, string, time.Time) (domain.User, error) {
				called = true
				return domain.User{ID: testUserID}, nil
			}, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
				called = true
				return service.AuthResult{}, nil
			}}, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := httptest.NewRequest(http.MethodPost, path, nil).WithContext(ctx)
			r.Body = io.NopCloser(&cancelingBody{reader: strings.NewReader(authBody), cancel: cancel})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			assert.Equal(t, 503, w.Code)
			assert.False(t, called)
			assert.Empty(t, w.Result().Cookies())
			assert.Empty(t, h.safety.hashing)
		})
	}
}

func TestLimiterConcurrentRequestsShareOneIPBucket(t *testing.T) {
	var called atomic.Int64
	h, router := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called.Add(1); return testUserID, nil }}, false)
	var accepted atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup
	for i := range 128 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := safetyRequest(router, http.MethodGet, "/auth/me", fmt.Sprintf("192.0.2.1:%d", 1000+i))
			switch w.Code {
			case 200:
				accepted.Add(1)
			case 429:
				rejected.Add(1)
			default:
				t.Errorf("unexpected status %d", w.Code)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(60), accepted.Load())
	assert.Equal(t, int64(68), rejected.Load())
	assert.Equal(t, int64(60), called.Load())
	assert.Len(t, h.safety.limiter.clients, 1)
}

func TestHashingSlotsReleaseOnEveryResponsePath(t *testing.T) {
	for _, mode := range []string{"invalid body", "service error", "service panic"} {
		t.Run(mode, func(t *testing.T) {
			s := &fakeAuthService{login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
				if mode == "service panic" {
					panic("test service failure")
				}
				return service.AuthResult{}, errors.New("test service failure")
			}}
			h, router := authRouter(s, false)
			for i := range 3 {
				body := authBody
				if mode == "invalid body" {
					body = "null"
				}
				r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
				r.RemoteAddr = fmt.Sprintf("192.0.2.%d:1", i)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				assert.NotEqual(t, 503, w.Code)
				assert.Empty(t, h.safety.hashing)
			}
		})
	}
}
