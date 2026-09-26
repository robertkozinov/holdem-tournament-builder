package http

import (
	"context"
	"encoding/json"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuthService struct {
	registerCalled bool
	registerName   string
	registerPass   string
	registerResult service.AuthResult
	registerErr    error

	loginCalled bool
	loginName   string
	loginPass   string
	loginResult service.AuthResult
	loginErr    error

	authenticateCalled bool
	authenticateToken  string
	authenticateUser   *domain.User
	authenticateErr    error

	logoutCalled bool
	logoutToken  string
	logoutErr    error
}

func (s *mockAuthService) Register(_ context.Context, username, password string, _ time.Time) (service.AuthResult, error) {
	s.registerCalled = true
	s.registerName = username
	s.registerPass = password
	return s.registerResult, s.registerErr
}

func (s *mockAuthService) Login(_ context.Context, username, password string, _ time.Time) (service.AuthResult, error) {
	s.loginCalled = true
	s.loginName = username
	s.loginPass = password
	return s.loginResult, s.loginErr
}

func (s *mockAuthService) Authenticate(_ context.Context, rawToken string, _ time.Time) (*domain.User, error) {
	s.authenticateCalled = true
	s.authenticateToken = rawToken
	return s.authenticateUser, s.authenticateErr
}

func (s *mockAuthService) Logout(_ context.Context, rawToken string) error {
	s.logoutCalled = true
	s.logoutToken = rawToken
	return s.logoutErr
}

func authRouter(s *mockAuthService, secureCookie bool) http.Handler {
	return NewRouter(NewTournamentHandler(&mockTournamentService{}), NewAuthHandler(s, secureCookie))
}

func TestAuthHandler_Register(t *testing.T) {
	t.Run("creates user and sets session cookie", func(t *testing.T) {
		id := uuid.New()
		expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
		svc := &mockAuthService{registerResult: service.AuthResult{
			UserID: id, Username: "table_host", Token: "raw-token", ExpiresAt: expiresAt,
		}}
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"table_host","password":"RiverCard7!"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		authRouter(svc, true).ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "table_host", svc.registerName)
		assert.Equal(t, "RiverCard7!", svc.registerPass)
		var body authUserResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, id, body.ID)
		assert.Equal(t, "table_host", body.Username)
		assert.NotContains(t, rec.Body.String(), "raw-token")
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, sessionCookieName, cookies[0].Name)
		assert.Equal(t, "raw-token", cookies[0].Value)
		assert.Equal(t, "/", cookies[0].Path)
		assert.True(t, cookies[0].HttpOnly)
		assert.True(t, cookies[0].Secure)
		assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
		assert.True(t, expiresAt.Equal(cookies[0].Expires))
	})

	t.Run("rejects malformed requests before calling service", func(t *testing.T) {
		cases := []struct {
			name        string
			contentType string
			body        string
		}{
			{name: "wrong content type", contentType: "text/plain", body: `{}`},
			{name: "bad json", contentType: "application/json", body: `{"username":`},
			{name: "unknown field", contentType: "application/json", body: `{"username":"a","password":"b","admin":true}`},
			{name: "multiple objects", contentType: "application/json", body: `{} {}`},
			{name: "oversized body", contentType: "application/json", body: strings.Repeat(" ", maxAuthRequestBytes) + `{}`},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockAuthService{}
				req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(tt.body))
				req.Header.Set("Content-Type", tt.contentType)
				rec := httptest.NewRecorder()

				authRouter(svc, false).ServeHTTP(rec, req)

				assert.Equal(t, http.StatusBadRequest, rec.Code)
				assert.False(t, svc.registerCalled)
				assert.Empty(t, rec.Result().Cookies())
			})
		}
	})

	t.Run("maps duplicate username to conflict", func(t *testing.T) {
		svc := &mockAuthService{registerErr: app.ErrUsernameAlreadyExists}
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"table_host","password":"RiverCard7!"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Empty(t, rec.Result().Cookies())
	})

	t.Run("maps validation error to bad request", func(t *testing.T) {
		svc := &mockAuthService{registerErr: errors.Join(app.ErrValidation, app.ErrInvalidPassword)}
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"table_host","password":"short"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid password")
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("sets cookie on successful login", func(t *testing.T) {
		id := uuid.New()
		svc := &mockAuthService{loginResult: service.AuthResult{
			UserID: id, Username: "table_host", Token: "raw-token", ExpiresAt: time.Now().Add(time.Hour),
		}}
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"table_host","password":"RiverCard7!"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "table_host", svc.loginName)
		assert.Equal(t, "RiverCard7!", svc.loginPass)
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "raw-token", cookies[0].Value)
		assert.False(t, cookies[0].Secure)
		assert.NotContains(t, rec.Body.String(), "raw-token")
	})

	t.Run("invalid credentials return unauthorized without cookie", func(t *testing.T) {
		svc := &mockAuthService{loginErr: app.ErrInvalidCredentials}
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"table_host","password":"wrong123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Empty(t, rec.Result().Cookies())
	})
}

func TestAuthHandler_Me(t *testing.T) {
	t.Run("returns current user without password hash", func(t *testing.T) {
		user := &domain.User{ID: uuid.New(), Username: "table_host", PasswordHash: "secret-hash"}
		svc := &mockAuthService{authenticateUser: user}
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, svc.authenticateCalled)
		assert.Equal(t, "raw-token", svc.authenticateToken)
		var body authUserResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, user.ID, body.ID)
		assert.Equal(t, user.Username, body.Username)
		assert.NotContains(t, rec.Body.String(), "secret-hash")
	})

	t.Run("missing or expired session returns unauthorized", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			withCookie bool
		}{
			{name: "missing cookie"},
			{name: "expired token", withCookie: true},
		} {
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockAuthService{authenticateErr: app.ErrUnauthenticated}
				req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
				if tt.withCookie {
					req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-token"})
				}
				rec := httptest.NewRecorder()

				authRouter(svc, false).ServeHTTP(rec, req)

				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, tt.withCookie, svc.authenticateCalled)
			})
		}
	})

	t.Run("repository failure returns server error", func(t *testing.T) {
		svc := &mockAuthService{authenticateErr: errors.New("database unavailable")}
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotContains(t, rec.Body.String(), "database unavailable")
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	t.Run("deletes session and clears cookie", func(t *testing.T) {
		svc := &mockAuthService{}
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
		rec := httptest.NewRecorder()

		authRouter(svc, true).ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, svc.logoutCalled)
		assert.Equal(t, "raw-token", svc.logoutToken)
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, sessionCookieName, cookies[0].Name)
		assert.Equal(t, -1, cookies[0].MaxAge)
		assert.Equal(t, "/", cookies[0].Path)
		assert.True(t, cookies[0].Secure)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("missing cookie is idempotent", func(t *testing.T) {
		svc := &mockAuthService{}
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, svc.logoutCalled)
		assert.Empty(t, svc.logoutToken)
	})

	t.Run("database failure does not clear cookie", func(t *testing.T) {
		svc := &mockAuthService{logoutErr: errors.New("database unavailable")}
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
		rec := httptest.NewRecorder()

		authRouter(svc, false).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Empty(t, rec.Result().Cookies())
	})
}

func TestAuthMiddleware_PassesAuthenticatedUserToNextHandler(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Username: "table_host"}
	svc := &mockAuthService{authenticateUser: user}
	authHandler := NewAuthHandler(svc, false)
	var currentUser *domain.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser = CurrentUser(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
	rec := httptest.NewRecorder()

	authHandler.RequireAuth(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Same(t, user, currentUser)
	assert.Equal(t, "raw-token", svc.authenticateToken)
}

func TestAuthMiddleware_DoesNotCallNextWithoutUser(t *testing.T) {
	svc := &mockAuthService{}
	authHandler := NewAuthHandler(svc, false)
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "raw-token"})
	rec := httptest.NewRecorder()

	authHandler.RequireAuth(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}
