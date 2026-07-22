package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type mockTournamentService struct {
	createCalled bool
	createInput  service.CreateTournamentInput
	createID     uuid.UUID
	createErr    error

	getCalled bool
	getID     uuid.UUID
	getTr     *domain.Tournament
	getErr    error

	deleteCalled bool
	deleteID     uuid.UUID
	deleteErr    error
}

func (s *mockTournamentService) CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error) {
	s.createCalled = true
	s.createInput = input

	if s.createErr != nil {
		return uuid.Nil, s.createErr
	}

	return s.createID, nil
}

func (s *mockTournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	s.getCalled = true
	s.getID = id

	if s.getErr != nil {
		return nil, s.getErr
	}

	return s.getTr, nil
}

func (s *mockTournamentService) DeleteTournament(ctx context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.deleteID = id

	if s.deleteErr != nil {
		return s.deleteErr
	}

	return nil
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

func validTournamentWithID(t *testing.T, id uuid.UUID) *domain.Tournament {
	t.Helper()

	tr, err := domain.NewTournament(
		"Friday Game",
		time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC),
		[]string{"A", "B"},
		1000,
		[]domain.ChipDenomination{{Value: 25, Count: 100}},
		domain.RebuyRules{Allowed: true, MaxLevel: 3},
		2*time.Hour,
		domain.StackPlan{
			Distribution: []domain.ChipDenomination{
				{Value: 25, Count: 20},
				{Value: 100, Count: 10},
			},
		},
		[]domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Ante: 0, Duration: 20 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Ante: 0, Duration: 20 * time.Minute},
		},
		[]domain.PayoutSpot{
			{Place: 1, Kind: domain.PayoutRemainder},
		},
	)
	require.NoError(t, err)

	tr.ID = id
	return tr
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

func TestTournamentHandler_GetTournamentByID(t *testing.T) {
	t.Run("gets tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			getTr: validTournamentWithID(t, id),
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodGet,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, srv.getCalled)
		assert.Equal(t, id, srv.getID)

		var body TournamentResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

		assert.Equal(t, id.String(), body.ID)
		assert.Equal(t, "Friday Game", body.Name)
		assert.Equal(t, "created", body.Status)
		assert.Equal(t, 120, body.DurationMinutes)
		assert.Equal(t, int64(2000), body.Pot)
		assert.Equal(t, int64(1500), body.StartingStack.Total)
		require.Len(t, body.BlindStructure, 2)
		require.NotNil(t, body.CurrentBlindLevel)
		require.NotNil(t, body.NextBlindLevel)
		assert.Nil(t, body.LevelStartedAt)
		assert.Nil(t, body.PausedAt)
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodGet,
			"/tournaments/invalid-id",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.getCalled)
	})

	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			getErr: app.ErrTournamentNotFound,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodGet,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.True(t, srv.getCalled)
		assert.Equal(t, id, srv.getID)
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			getErr: errors.New("service error"),
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodGet,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.True(t, srv.getCalled)
	})
}

func TestTournamentHandler_DeleteTournament(t *testing.T) {
	t.Run("deletes tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodDelete,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.deleteCalled)
		assert.Equal(t, id, srv.deleteID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodDelete,
			"/tournaments/invalid-id",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.deleteCalled)
	})

	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			deleteErr: app.ErrTournamentNotFound,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodDelete,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.True(t, srv.deleteCalled)
		assert.Equal(t, id, srv.deleteID)
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			deleteErr: errors.New("service error"),
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodDelete,
			"/tournaments/"+id.String(),
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.True(t, srv.deleteCalled)
	})
}
