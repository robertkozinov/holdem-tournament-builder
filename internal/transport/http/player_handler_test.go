package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type playerServiceStub struct {
	called        bool
	name, query   string
	id            uuid.UUID
	limit, offset int
	err           error
	players       []domain.Player
}

func (s *playerServiceStub) CreatePlayer(_ context.Context, name string, _ time.Time) (domain.Player, error) {
	s.called, s.name = true, name
	return domain.Player{ID: uuid.New(), Name: name}, s.err
}
func (s *playerServiceStub) ListPlayers(_ context.Context, query string, limit, offset int) ([]domain.Player, error) {
	s.called, s.query, s.limit, s.offset = true, query, limit, offset
	return s.players, s.err
}
func (s *playerServiceStub) RenamePlayer(_ context.Context, id uuid.UUID, name string, _ time.Time) (domain.Player, error) {
	s.called, s.id, s.name = true, id, name
	return domain.Player{ID: id, Name: name}, s.err
}
func (s *playerServiceStub) DeletePlayer(_ context.Context, id uuid.UUID, _ time.Time) error {
	s.called, s.id = true, id
	return s.err
}

func playerHandlerRequest(handler http.HandlerFunc, method, path, body, playerID string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("player_id", playerID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestPlayerHandler_CRUD(t *testing.T) {
	s := &playerServiceStub{}
	h := NewPlayerHandler(s)
	created := playerHandlerRequest(h.CreatePlayer, "POST", "/players", `{"name":"Влад"}`, "")
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, "Влад", s.name)
	assert.Contains(t, created.Body.String(), `"name":"Влад"`)
	assert.NotContains(t, created.Body.String(), "normalized_name")
	listed := playerHandlerRequest(h.ListPlayers, "GET", "/players?q=Фед&limit=10&offset=20", "", "")
	require.Equal(t, http.StatusOK, listed.Code)
	assert.JSONEq(t, "[]", listed.Body.String())
	assert.Equal(t, "Фед", s.query)
	assert.Equal(t, 10, s.limit)
	assert.Equal(t, 20, s.offset)
	id := uuid.New()
	renamed := playerHandlerRequest(h.RenamePlayer, "PATCH", "/players/"+id.String(), `{"name":"Влад Фед"}`, id.String())
	require.Equal(t, http.StatusOK, renamed.Code)
	assert.Equal(t, id, s.id)
	assert.Equal(t, "Влад Фед", s.name)
	deleted := playerHandlerRequest(h.DeletePlayer, "DELETE", "/players/"+id.String(), "", id.String())
	require.Equal(t, http.StatusNoContent, deleted.Code)
	assert.Empty(t, deleted.Body.String())
}

func TestPlayerHandler_InvalidRequests(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"name":null}`, `{"name":1}`, `{"name":"Влад","user_id":"fake"}`, `{"name":"Влад"} {}`, "{", "{\"name\":\"A\xffB\"}", `{"name":"` + strings.Repeat("a", 5000) + `"}`} {
		t.Run(body[:min(len(body), 30)], func(t *testing.T) {
			s := &playerServiceStub{}
			w := playerHandlerRequest(NewPlayerHandler(s).CreatePlayer, "POST", "/players", body, "")
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge}, w.Code)
			assert.False(t, s.called)
		})
	}
	for _, path := range []string{"/players?limit=0", "/players?limit=101", "/players?offset=-1", "/players?q=a&q=b", "/players?limit=1&limit=2", "/players?q=%ZZ"} {
		s := &playerServiceStub{}
		w := playerHandlerRequest(NewPlayerHandler(s).ListPlayers, "GET", path, "", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, s.called)
	}
	for _, id := range []string{"no-uuid", uuid.Nil.String()} {
		s := &playerServiceStub{}
		h := NewPlayerHandler(s)
		assert.Equal(t, http.StatusBadRequest, playerHandlerRequest(h.RenamePlayer, "PATCH", "/players/"+id, `{"name":"Влад"}`, id).Code)
		assert.Equal(t, http.StatusBadRequest, playerHandlerRequest(h.DeletePlayer, "DELETE", "/players/"+id, "", id).Code)
		assert.False(t, s.called)
	}
}

func TestPlayerHandler_RenameRejectsInvalidUTF8(t *testing.T) {
	s := &playerServiceStub{}
	w := playerHandlerRequest(NewPlayerHandler(s).RenamePlayer, "PATCH", "/players/id", "{\"name\":\"A\xffB\"}", uuid.NewString())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, s.called)
}

func TestPlayerHandler_Errors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{app.ErrUnauthenticated, http.StatusUnauthorized},
		{app.ErrPlayerNotFound, http.StatusNotFound},
		{app.ErrPlayerNameTaken, http.StatusConflict},
		{app.ErrValidation, http.StatusBadRequest},
		{errors.New("private database error"), http.StatusInternalServerError},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			h := NewPlayerHandler(&playerServiceStub{err: tc.err})
			for _, w := range []*httptest.ResponseRecorder{
				playerHandlerRequest(h.CreatePlayer, "POST", "/players", `{"name":"Влад"}`, ""),
				playerHandlerRequest(h.ListPlayers, "GET", "/players", "", ""),
				playerHandlerRequest(h.RenamePlayer, "PATCH", "/players/id", `{"name":"Влад"}`, uuid.NewString()),
				playerHandlerRequest(h.DeletePlayer, "DELETE", "/players/id", "", uuid.NewString()),
			} {
				assert.Equal(t, tc.status, w.Code)
				assert.NotContains(t, w.Body.String(), "private database")
			}
		})
	}
}

func TestPlayerRoutes_Protected(t *testing.T) {
	id := uuid.NewString()
	for _, route := range []struct{ method, path, body string }{
		{"POST", "/players", `{"name":"Влад"}`},
		{"GET", "/players?q=влад", ""},
		{"PATCH", "/players/" + id, `{"name":"Влад Фед"}`},
		{"DELETE", "/players/" + id, ""},
	} {
		t.Run(route.method, func(t *testing.T) {
			for _, tc := range []struct {
				name, cookie, origin string
				authErr              error
				want                 int
			}{
				{"no cookie", "", "", nil, 401},
				{"expired", "token", "", app.ErrUnauthenticated, 401},
				{"redis down", "token", "", app.ErrSessionUnavailable, 503},
				{"cross origin", "token", "https://foreign.example", nil, 403},
				{"authenticated", "token", "", nil, map[string]int{"POST": 201, "GET": 200, "PATCH": 200, "DELETE": 204}[route.method]},
			} {
				if route.method == "GET" && tc.origin != "" {
					continue
				}
				t.Run(tc.name, func(t *testing.T) {
					ps := &playerServiceStub{}
					accounts := &fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { return testUserID, tc.authErr }}
					router := NewRouter(NewTournamentHandler(&mockTournamentService{}), NewAuthHandler(accounts, CookieOptions{}, config.DefaultRequestSafety()), NewPlayerHandler(ps))
					r := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					if tc.cookie != "" {
						r.AddCookie(&http.Cookie{Name: "holdem_session", Value: tc.cookie})
					}
					if tc.origin != "" {
						r.Header.Set("Origin", tc.origin)
					}
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					assert.Equal(t, tc.want, w.Code)
					assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
					assert.Equal(t, tc.want < 400, ps.called)
				})
			}
		})
	}
}

func TestPlayerRoutes_RateLimitBeforeSessionLookup(t *testing.T) {
	count := 0
	accounts := &fakeAuthService{authenticate: func(context.Context, string, time.Time) (uuid.UUID, error) { count++; return testUserID, nil }}
	safety := config.DefaultRequestSafety()
	safety.General.Burst = 1
	h := NewAuthHandler(accounts, CookieOptions{}, safety)
	h.now = func() time.Time { return time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC) }
	router := NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(&playerServiceStub{}))
	for _, status := range []int{200, 429} {
		r := httptest.NewRequest("GET", "/players", nil)
		r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "token"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		assert.Equal(t, status, w.Code)
	}
	assert.Equal(t, 1, count)
}

func TestRouterRequiresPlayerHandler(t *testing.T) {
	h := NewAuthHandler(&fakeAuthService{}, CookieOptions{}, config.DefaultRequestSafety())
	assert.Panics(t, func() { NewRouter(NewTournamentHandler(&mockTournamentService{}), h, nil) })
	assert.Panics(t, func() { NewRouter(NewTournamentHandler(&mockTournamentService{}), h, NewPlayerHandler(nil)) })
}
