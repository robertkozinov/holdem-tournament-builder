package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Run("serves frontend", func(t *testing.T) {
		handler := NewTournamentHandler(&mockTournamentService{})
		router := NewRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Holdem Tournament Builder")
	})

	t.Run("registers health route", func(t *testing.T) {
		handler := NewTournamentHandler(&mockTournamentService{})
		router := NewRouter(handler)

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
		router := NewRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON()))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		assert.True(t, service.createCalled)
	})
}
