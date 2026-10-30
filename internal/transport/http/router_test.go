package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"holdem-tournament-builder/internal/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Run("registers health route", func(t *testing.T) {
		handler := NewTournamentHandler(&mockTournamentService{})
		router := newTournamentTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())
	})

	t.Run("registers create tournament route", func(t *testing.T) {
		id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
		service := &mockTournamentService{createID: id}
		handler := NewTournamentHandler(service)
		router := newTournamentTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON()))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		assert.True(t, service.createCalled)
	})

	t.Run("does not register removed runtime routes", func(t *testing.T) {
		router := newTournamentTestRouter(NewTournamentHandler(&mockTournamentService{}))
		id := uuid.New().String()

		for _, path := range []string{"start", "pause", "resume", "level-up", "knockout", "finish", "rebuy", "preview"} {
			t.Run(path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/tournaments/"+id+"/"+path, nil)
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusNotFound, rec.Code)
			})
		}
	})
}

func TestRouterRequiresStatisticsHandler(t *testing.T) {
	auth := NewAuthHandler(&fakeAuthService{}, CookieOptions{}, config.DefaultRequestSafety())
	tournaments := NewTournamentHandler(&mockTournamentService{})
	players := NewPlayerHandler(&playerServiceStub{})

	assert.Panics(t, func() { NewRouter(tournaments, auth, players, nil) })
	assert.Panics(t, func() { NewRouter(tournaments, auth, players, NewStatisticsHandler(nil)) })
}
