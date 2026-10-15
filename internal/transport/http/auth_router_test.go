package http

import (
	"context"
	"encoding/json"
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
	"holdem-tournament-builder/internal/service"
)

func protectedRoutes() []struct{ method, path string } {
	id := uuid.New().String()
	return []struct{ method, path string }{
		{http.MethodPost, "/tournaments"}, {http.MethodGet, "/tournaments"},
		{http.MethodGet, "/tournaments/" + id}, {http.MethodPut, "/tournaments/" + id},
		{http.MethodPatch, "/tournaments/" + id + "/players/" + uuid.New().String()},
		{http.MethodDelete, "/tournaments/" + id}, {http.MethodGet, "/auth/me"},
	}
}

func TestProtectedRoutesRejectInvalidCookiesBeforeService(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "holdem_session"
		if secure {
			name = "__Host-holdem_session"
		}
		for _, header := range []string{"", name + "=", name + "=one; " + name + "=two", name + "=; " + name + "=one", "unrelated=one"} {
			for _, route := range protectedRoutes() {
				t.Run(fmt.Sprint(secure)+header+route.method+route.path, func(t *testing.T) {
					called := false
					h, _ := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called = true; return testUserID, nil }}, secure)
					s := &mockTournamentService{}
					router := NewRouter(NewTournamentHandler(s), h, NewPlayerHandler(&playerServiceStub{}))
					r := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
					r.Header.Set("Cookie", header)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					assert.Equal(t, http.StatusUnauthorized, w.Code)
					assert.False(t, called)
					assert.False(t, s.createCalled || s.getCalled || s.saveCalled || s.patchCalled || s.deleteCalled || s.listCalled)
				})
			}
		}
	}
}

func TestSessionFailuresNeverReachProtectedHandler(t *testing.T) {
	for _, tt := range []struct {
		name   string
		id     uuid.UUID
		err    error
		status int
	}{
		{"expired", uuid.Nil, app.ErrUnauthenticated, 401},
		{"unavailable", uuid.Nil, fmt.Errorf("private redis detail: %w", app.ErrSessionUnavailable), 503},
		{"invalid identity", uuid.Nil, nil, 401},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, route := range protectedRoutes() {
				h, _ := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { return tt.id, tt.err }}, false)
				s := &mockTournamentService{}
				w := authRequest(NewRouter(NewTournamentHandler(s), h, NewPlayerHandler(&playerServiceStub{})), route.method, route.path, "{}", &http.Cookie{Name: "holdem_session", Value: "test-session"})
				assert.Equal(t, tt.status, w.Code)
				assert.NotContains(t, w.Body.String(), "private redis detail")
				assert.False(t, s.createCalled || s.getCalled || s.saveCalled || s.patchCalled || s.deleteCalled || s.listCalled)
			}
		})
	}
}

func TestUnknownAndRemovedRoutesRemainPublic404(t *testing.T) {
	called := false
	_, router := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called = true; return testUserID, nil }}, false)
	for _, path := range []string{"/unknown", "/auth/unknown", "/tournaments/one/start", "/tournaments/one/pause", "/tournaments/one/resume", "/tournaments/one/level-up", "/tournaments/one/knockout", "/tournaments/one/finish", "/tournaments/one/rebuy", "/tournaments/one/preview"} {
		w := authRequest(router, http.MethodPost, path, "")
		assert.Equal(t, 404, w.Code, path)
	}
	w := authRequest(router, http.MethodGet, "/health", "")
	assert.Equal(t, 200, w.Code)
	assert.False(t, called)
}

func TestRouterRequiresAuthHandler(t *testing.T) {
	assert.Panics(t, func() {
		NewRouter(NewTournamentHandler(&mockTournamentService{}), nil, NewPlayerHandler(&playerServiceStub{}))
	})
}

func TestProtectedTournamentResponsesAreNotCached(t *testing.T) {
	router := NewRouter(NewTournamentHandler(&mockTournamentService{}), NewAuthHandler(&fakeAuthService{}, CookieOptions{}, config.DefaultRequestSafety()), NewPlayerHandler(&playerServiceStub{}))
	for _, tc := range []struct {
		name    string
		cookies []*http.Cookie
		status  int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"authenticated", []*http.Cookie{{Name: "holdem_session", Value: "test-session"}}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := authRequest(router, http.MethodGet, "/tournaments", "", tc.cookies...)
			assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			assert.Equal(t, tc.status, w.Code)
		})
	}
}

func TestCrossOriginUnsafeRoutes(t *testing.T) {
	for _, route := range append(protectedRoutes()[:6], []struct{ method, path string }{{http.MethodPost, "/auth/register"}, {http.MethodPost, "/auth/login"}, {http.MethodPost, "/auth/logout"}}...) {
		if route.method == http.MethodGet {
			continue
		}
		for _, header := range []string{"Origin", "Sec-Fetch-Site"} {
			t.Run(route.method+route.path+header, func(t *testing.T) {
				called := false
				h, _ := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called = true; return testUserID, nil }}, false)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(route.method, route.path, strings.NewReader(authBody))
				r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "test-session"})
				value := "https://foreign.example"
				if header == "Sec-Fetch-Site" {
					value = "cross-site"
				}
				r.Header.Set(header, value)
				NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{})).ServeHTTP(w, r)
				require.Equal(t, 403, w.Code)
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.False(t, called)
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			})
		}
	}
	_, router := authRouter(&fakeAuthService{}, false)
	r := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(authBody))
	r.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	assert.Equal(t, 201, w.Code)
}

func TestTournamentUnauthenticatedError(t *testing.T) {
	w := serve(t, &mockTournamentService{listErr: fmt.Errorf("private: %w", app.ErrUnauthenticated)}, http.MethodGet, "/tournaments", "")
	assert.Equal(t, 401, w.Code)
	assert.NotContains(t, w.Body.String(), "private")
}

func TestDuplicateCookiesWithMalformedValueAreRejected(t *testing.T) {
	for _, path := range []string{"/auth/me", "/auth/logout", "/tournaments"} {
		for _, separateHeaders := range []bool{false, true} {
			t.Run(path+fmt.Sprint(separateHeaders), func(t *testing.T) {
				called := false
				_, router := authRouter(&fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { called = true; return testUserID, nil }, logout: func(context.Context, string) error { called = true; return nil }}, false)
				method := http.MethodGet
				if path == "/auth/logout" {
					method = http.MethodPost
				}
				r := httptest.NewRequest(method, path, nil)
				if separateHeaders {
					r.Header.Add("Cookie", "holdem_session=one")
					r.Header.Add("Cookie", `holdem_session=bad"value`)
				} else {
					r.Header.Set("Cookie", `holdem_session=one; holdem_session=bad"value`)
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				assert.Equal(t, 401, w.Code)
				assert.False(t, called)
				assert.Empty(t, w.Result().Cookies())
			})
		}
	}
}

func TestRouterOwnerIsolationWithRealTournamentService(t *testing.T) {
	otherID := uuid.New()
	s := &fakeAuthService{authenticate: func(_ context.Context, token string, _ time.Time) (uuid.UUID, error) {
		if token == "owner-one" {
			return testUserID, nil
		}
		if token == "owner-two" {
			return otherID, nil
		}
		return uuid.Nil, app.ErrUnauthenticated
	}}
	h, _ := authRouter(s, false)
	repo := &memoryTournamentRepository{cards: make(map[uuid.UUID][]byte)}
	router := NewRouter(NewTournamentHandler(service.NewTournamentService(repo)), h, NewPlayerHandler(&playerServiceStub{}))
	request := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := authRequest(router, method, path, body, &http.Cookie{Name: "holdem_session", Value: token})
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	created := request(http.MethodPost, "/tournaments", validCreateTournamentJSON(), "owner-one", 201)
	var first CreateTournamentResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &first))
	tr, err := repo.GetByID(context.Background(), testUserID, uuid.MustParse(first.ID))
	require.NoError(t, err)
	before := append([]byte(nil), repo.cards[tr.ID]...)
	secondCreated := request(http.MethodPost, "/tournaments", validCreateTournamentJSON(), "owner-two", 201)
	var second CreateTournamentResponse
	require.NoError(t, json.Unmarshal(secondCreated.Body.Bytes(), &second))
	path := "/tournaments/" + first.ID
	settings, err := json.Marshal(SaveTournamentRequest{TournamentSettingsRequest: settingsRequestFixture()})
	require.NoError(t, err)
	request(http.MethodGet, path, "", "owner-two", 404)
	request(http.MethodPut, path, string(settings), "owner-two", 404)
	request(http.MethodPatch, path+"/players/"+tr.Players[0].ID.String(), `{"buy_in_paid":true}`, "owner-two", 404)
	request(http.MethodDelete, path, "", "owner-two", 404)
	listed := request(http.MethodGet, "/tournaments", "", "owner-two", 200)
	var cards []TournamentResponse
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &cards))
	require.Len(t, cards, 1)
	assert.Equal(t, second.ID, cards[0].ID)
	assert.Equal(t, before, repo.cards[tr.ID])
	request(http.MethodGet, path, "", "owner-one", 200)
}
