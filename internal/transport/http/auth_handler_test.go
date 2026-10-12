package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
)

const authBody = `{"login":"organizer","password":"test-password"}`

func authRequest(router http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func authRouter(s *fakeAuthService, secure bool) (*AuthHandler, http.Handler) {
	h := NewAuthHandler(s, CookieOptions{Secure: secure}, config.DefaultRequestSafety())
	h.now = func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC) }
	return h, NewRouter(NewTournamentHandler(&mockTournamentService{}), h)
}

func TestAuthEndpointsAndCookies(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint(secure), func(t *testing.T) {
			h, router := authRouter(&fakeAuthService{}, secure)
			registered := authRequest(router, http.MethodPost, "/auth/register", authBody)
			require.Equal(t, http.StatusCreated, registered.Code)
			assert.JSONEq(t, fmt.Sprintf(`{"id":%q,"login":"organizer"}`, testUserID), registered.Body.String())
			assert.Empty(t, registered.Result().Cookies())
			logged := authRequest(router, http.MethodPost, "/auth/login", authBody)
			require.Equal(t, http.StatusOK, logged.Code)
			assert.JSONEq(t, registered.Body.String(), logged.Body.String())
			cookies := logged.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie := cookies[0]
			name := "holdem_session"
			if secure {
				name = "__Host-holdem_session"
			}
			assert.Equal(t, name, cookie.Name)
			assert.Equal(t, secure, cookie.Secure)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Equal(t, "/", cookie.Path)
			assert.Empty(t, cookie.Domain)
			assert.Equal(t, 30*24*60*60, cookie.MaxAge)
			assert.Equal(t, h.now().Add(30*24*time.Hour), cookie.Expires)
			me := authRequest(router, http.MethodGet, "/auth/me", "", cookie)
			require.Equal(t, http.StatusOK, me.Code)
			assert.JSONEq(t, registered.Body.String(), me.Body.String())
			assert.Empty(t, me.Result().Cookies())
			logout := authRequest(router, http.MethodPost, "/auth/logout", "", cookie)
			require.Equal(t, http.StatusNoContent, logout.Code)
			require.Len(t, logout.Result().Cookies(), 1)
			cleared := logout.Result().Cookies()[0]
			assert.Equal(t, cookie.Name, cleared.Name)
			assert.Equal(t, cookie.Secure, cleared.Secure)
			assert.Equal(t, cookie.Path, cleared.Path)
			assert.True(t, cleared.HttpOnly)
			assert.Equal(t, cookie.SameSite, cleared.SameSite)
			assert.Empty(t, cleared.Domain)
			assert.Empty(t, cleared.Value)
			assert.Less(t, cleared.MaxAge, 0)
			for _, w := range []*httptest.ResponseRecorder{registered, logged, me, logout} {
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				for _, secret := range []string{"password", "hash", "token", "expires", "private-hash", "test-session"} {
					assert.NotContains(t, w.Body.String(), secret)
				}
			}
		})
	}
}

func TestAuthStrictBodies(t *testing.T) {
	for _, path := range []string{"/auth/register", "/auth/login"} {
		for _, body := range []string{"", "null", "[]", "{", authBody + "{}", authBody + "x", `{"login":null,"password":"test-password"}`, `{"login":"organizer","password":null}`, `{"login":1,"password":"test-password"}`, `{"login":"organizer","password":true}`, `{"login":"organizer"}`, `{}`, `{"login":"organizer","password":"test-password","extra":1}`} {
			t.Run(path+body, func(t *testing.T) {
				called := false
				s := &fakeAuthService{register: func(context.Context, string, string, time.Time) (domain.User, error) {
					called = true
					return domain.User{}, nil
				}, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
					called = true
					return service.AuthResult{}, nil
				}}
				_, router := authRouter(s, false)
				w := authRequest(router, http.MethodPost, path, body)
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.False(t, called)
				assert.Empty(t, w.Result().Cookies())
			})
		}
		for _, body := range []string{`{"login":"` + strings.Repeat("a", 4096) + `","password":"test-password"}`, authBody + strings.Repeat(" ", 4096)} {
			t.Run(path+"oversized", func(t *testing.T) {
				_, router := authRouter(&fakeAuthService{}, false)
				w := authRequest(router, http.MethodPost, path, body)
				assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			})
		}
	}
}

func TestAuthErrorMapping(t *testing.T) {
	for _, tt := range []struct {
		err     error
		status  int
		message string
	}{
		{fmt.Errorf("private-detail: %w", app.ErrValidation), 400, "invalid request"},
		{fmt.Errorf("private-detail: %w", app.ErrLoginTaken), 409, "login is already taken"},
		{fmt.Errorf("private-detail: %w", app.ErrInvalidCredentials), 401, "invalid login or password"},
		{fmt.Errorf("private-detail: %w", app.ErrUnauthenticated), 401, "authentication required"},
		{fmt.Errorf("private-detail: %w", app.ErrSessionUnavailable), 503, "authentication temporarily unavailable"},
		{errors.New("private-detail"), 500, "internal server error"},
	} {
		t.Run(tt.message, func(t *testing.T) {
			for _, path := range []string{"/auth/register", "/auth/login", "/auth/logout", "/auth/me"} {
				s := &fakeAuthService{register: func(context.Context, string, string, time.Time) (domain.User, error) { return domain.User{}, tt.err }, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
					return service.AuthResult{}, tt.err
				}, logout: func(context.Context, string) error { return tt.err }, currentUser: func(context.Context, uuid.UUID) (domain.User, error) { return domain.User{}, tt.err }}
				_, router := authRouter(s, false)
				method := http.MethodPost
				if path == "/auth/me" {
					method = http.MethodGet
				}
				w := authRequest(router, method, path, authBody, &http.Cookie{Name: "holdem_session", Value: "test-session"})
				assert.Equal(t, tt.status, w.Code, path)
				assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, tt.message), w.Body.String())
				assert.Empty(t, w.Result().Cookies())
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestLogoutAbsentEmptyMalformedAndDuplicateCookies(t *testing.T) {
	for _, tt := range []struct {
		name, header  string
		status, calls int
	}{
		{"absent", "", 204, 0},
		{"empty", "holdem_session=", 204, 0},
		{"malformed token", "holdem_session=not-a-valid-token", 204, 1},
		{"duplicate", "holdem_session=one; holdem_session=two", 401, 0},
		{"duplicate empty", "holdem_session=; holdem_session=two", 401, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			_, router := authRouter(&fakeAuthService{logout: func(context.Context, string) error { calls++; return nil }}, false)
			r := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
			r.Header.Set("Cookie", tt.header)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			assert.Equal(t, tt.status, w.Code)
			assert.Equal(t, tt.calls, calls)
			if tt.status == 204 {
				require.Len(t, w.Result().Cookies(), 1)
				assert.Less(t, w.Result().Cookies()[0].MaxAge, 0)
			} else {
				assert.Empty(t, w.Result().Cookies())
			}
		})
	}
}

func TestAuthRejectsInvalidServiceResults(t *testing.T) {
	for _, tt := range []struct {
		name   string
		result service.AuthResult
	}{
		{"missing identity", service.AuthResult{Login: "organizer", Token: "test-session", ExpiresAt: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}},
		{"missing token", service.AuthResult{UserID: testUserID, Login: "organizer", ExpiresAt: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}},
		{"expired", service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC)}},
		{"invalid cookie", service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test;session", ExpiresAt: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, router := authRouter(&fakeAuthService{login: func(context.Context, string, string, time.Time) (service.AuthResult, error) { return tt.result, nil }}, false)
			w := authRequest(router, http.MethodPost, "/auth/login", authBody)
			assert.Equal(t, 500, w.Code)
			assert.Empty(t, w.Result().Cookies())
			assert.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
		})
	}
	_, router := authRouter(&fakeAuthService{currentUser: func(context.Context, uuid.UUID) (domain.User, error) {
		return domain.User{ID: uuid.New(), Login: "someone-else"}, nil
	}}, false)
	w := authRequest(router, http.MethodGet, "/auth/me", "", &http.Cookie{Name: "holdem_session", Value: "test-session"})
	assert.Equal(t, 401, w.Code)
	assert.NotContains(t, w.Body.String(), "someone-else")
}

func TestAuthPassesCredentialsAndClockUnchanged(t *testing.T) {
	for _, path := range []string{"/auth/register", "/auth/login"} {
		t.Run(path, func(t *testing.T) {
			var login, password string
			var at time.Time
			s := &fakeAuthService{register: func(_ context.Context, l, p string, now time.Time) (domain.User, error) {
				login, password, at = l, p, now
				return domain.User{ID: testUserID, Login: "organizer"}, nil
			}, login: func(_ context.Context, l, p string, now time.Time) (service.AuthResult, error) {
				login, password, at = l, p, now
				return service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: now.Add(30 * 24 * time.Hour)}, nil
			}}
			h, router := authRouter(s, false)
			w := authRequest(router, http.MethodPost, path, `{"login":" Organizer ","password":"PaSSword!"}`)
			assert.Less(t, w.Code, 400)
			assert.Equal(t, " Organizer ", login)
			assert.Equal(t, "PaSSword!", password)
			assert.Equal(t, h.now(), at)
		})
	}
}

func TestAuthRejectsAmbiguousCredentialFields(t *testing.T) {
	for _, body := range []string{
		`{"Login":"organizer","password":"test-password"}`,
		`{"login":"organizer","PASSWORD":"test-password"}`,
		`{"login":"organizer","login":"other-user","password":"test-password"}`,
		`{"login":"organizer","password":"test-password","password":"another-password"}`,
	} {
		for _, path := range []string{"/auth/register", "/auth/login"} {
			t.Run(path+body, func(t *testing.T) {
				called := false
				_, router := authRouter(&fakeAuthService{register: func(context.Context, string, string, time.Time) (domain.User, error) {
					called = true
					return domain.User{ID: testUserID}, nil
				}, login: func(context.Context, string, string, time.Time) (service.AuthResult, error) {
					called = true
					return service.AuthResult{}, nil
				}}, false)
				w := authRequest(router, http.MethodPost, path, body)
				assert.Equal(t, 400, w.Code)
				assert.False(t, called)
				assert.Empty(t, w.Result().Cookies())
			})
		}
	}
}
