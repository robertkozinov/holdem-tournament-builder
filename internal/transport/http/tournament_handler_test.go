package http

import (
	"context"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTournamentService struct {
	createCalled bool
	createInput  service.CreateTournamentInput
	createID     uuid.UUID
	createErr    error
}

func (s *mockTournamentService) CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error) {
	s.createCalled = true
	s.createInput = input

	if s.createErr != nil {
		return uuid.Nil, s.createErr
	}

	return s.createID, nil
}

func validCreateTournamentJSON() string {
	return `{
		"name": "Friday Game",
		"date": "2026-07-18T20:00:00Z",
		"players": ["A", "B", "C", "D"],
		"buy_in_amount": 1000,
		"chips": [
			{ "value": 25, "count": 80 },
			{ "value": 100, "count": 80 },
			{ "value": 500, "count": 40 }
		],
		"style": "standard",
		"duration_minutes": 120,
		"level_duration_minutes": 20,
		"rebuy": {
			"allowed": true,
			"max_level": 3
		},
		"payout": {
			"payout_mode": "default",
			"fixed_buy_ins": null
		}
	}`
}

func TestTournamentHandler_CreateTournament(t *testing.T) {
	t.Run("creates tournament", func(t *testing.T) {
		id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
		srv := &mockTournamentService{createID: id}
		handler := NewTournamentHandler(srv)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON()))
		rec := httptest.NewRecorder()

		handler.CreateTournament(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		assert.True(t, srv.createCalled)
		assert.Contains(t, rec.Body.String(), id.String())
	})

	t.Run("returns bad request when json is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		handler := NewTournamentHandler(srv)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(``))
		rec := httptest.NewRecorder()

		handler.CreateTournament(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.createCalled)
	})

	t.Run("returns bad request when style is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		handler := NewTournamentHandler(srv)

		body := strings.Replace(validCreateTournamentJSON(), `"style": "standard"`, `"style": "fast"`, 1)
		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.CreateTournament(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.createCalled)
	})

	t.Run("returns bad request when service returns validation error", func(t *testing.T) {
		srv := &mockTournamentService{
			createErr: fmt.Errorf("%w: invalid input", app.ErrValidation),
		}
		handler := NewTournamentHandler(srv)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON()))
		rec := httptest.NewRecorder()

		handler.CreateTournament(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.True(t, srv.createCalled)
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		srv := &mockTournamentService{
			createErr: errors.New("db down"),
		}
		handler := NewTournamentHandler(srv)

		req := httptest.NewRequest(http.MethodPost, "/tournaments", strings.NewReader(validCreateTournamentJSON()))
		rec := httptest.NewRecorder()

		handler.CreateTournament(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.True(t, srv.createCalled)
	})
}
