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

	startCalled bool
	startID     uuid.UUID
	startErr    error

	pauseCalled bool
	pauseID     uuid.UUID
	pauseErr    error

	resumeCalled bool
	resumeID     uuid.UUID
	resumeErr    error

	finishCalled bool
	finishID     uuid.UUID
	finishErr    error

	levelUpCalled bool
	levelUpID     uuid.UUID
	levelUpErr    error

	addRebuyCalled     bool
	addRebuyID         uuid.UUID
	addRebuyPlayerName string
	addRebuyErr        error

	knockoutCalled     bool
	knockoutID         uuid.UUID
	knockoutPlayerName string
	knockoutErr        error
}

func (s *mockTournamentService) CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error) {
	s.createCalled = true
	s.createInput = input

	if s.createErr != nil {
		return uuid.Nil, s.createErr
	}

	return s.createID, nil
}

func (s *mockTournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID, now time.Time) (*domain.Tournament, error) {
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

func (s *mockTournamentService) StartTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	s.startCalled = true
	s.startID = id

	if s.startErr != nil {
		return s.startErr
	}

	return nil
}

func (s *mockTournamentService) PauseTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	s.pauseCalled = true
	s.pauseID = id

	if s.pauseErr != nil {
		return s.pauseErr
	}

	return nil
}

func (s *mockTournamentService) ResumeTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	s.resumeCalled = true
	s.resumeID = id

	if s.resumeErr != nil {
		return s.resumeErr
	}

	return nil
}

func (s *mockTournamentService) FinishTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	s.finishCalled = true
	s.finishID = id

	if s.finishErr != nil {
		return s.finishErr
	}

	return nil
}

func (s *mockTournamentService) LevelUpTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	s.levelUpCalled = true
	s.levelUpID = id

	if s.levelUpErr != nil {
		return s.levelUpErr
	}

	return nil
}

func (s *mockTournamentService) AddRebuy(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error {
	s.addRebuyCalled = true
	s.addRebuyID = id
	s.addRebuyPlayerName = playerName

	if s.addRebuyErr != nil {
		return s.addRebuyErr
	}

	return nil
}

func (s *mockTournamentService) KnockoutPlayer(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error {
	s.knockoutCalled = true
	s.knockoutID = id
	s.knockoutPlayerName = playerName

	if s.knockoutErr != nil {
		return s.knockoutErr
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
		assert.Empty(t, body.Transfers)
	})

	t.Run("returns transfers for finished tournament", func(t *testing.T) {
		id := uuid.New()
		tr := validTournamentWithID(t, id)
		tr.Status = domain.StatusFinished
		tr.Players = []string{"A"}
		tr.Results = []domain.Result{
			{Name: "B", Place: 2, Prize: 0},
			{Name: "A", Place: 1, Prize: 2000},
		}

		srv := &mockTournamentService{getTr: tr}
		router := NewRouter(NewTournamentHandler(srv))
		req := httptest.NewRequest(http.MethodGet, "/tournaments/"+id.String(), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body TournamentResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, []TransferResponse{
			{From: "B", To: "A", Amount: 1000},
		}, body.Transfers)
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

func TestTournamentHandler_StartTournament(t *testing.T) {
	t.Run("starts tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/start",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.startCalled)
		assert.Equal(t, id, srv.startID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/invalid-id/start",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.startCalled)
	})

	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			startErr: app.ErrTournamentNotFound,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/start",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.True(t, srv.startCalled)
		assert.Equal(t, id, srv.startID)
	})

	t.Run("returns conflict when tournament cannot be started", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			startErr: domain.ErrIncorrectStatus,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/start",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.startCalled)
		assert.Equal(t, id, srv.startID)
	})

	t.Run("returns internal server error when service fails", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			startErr: errors.New("service error"),
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/start",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.True(t, srv.startCalled)
		assert.Equal(t, id, srv.startID)
	})
}

func TestTournamentHandler_PauseTournament(t *testing.T) {
	t.Run("pauses tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/pause",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.pauseCalled)
		assert.Equal(t, id, srv.pauseID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/invalid-id/pause",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.pauseCalled)
	})

	t.Run("returns conflict when tournament cannot be paused", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			pauseErr: domain.ErrIncorrectStatus,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/pause",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.pauseCalled)
		assert.Equal(t, id, srv.pauseID)
	})
}

func TestTournamentHandler_ResumeTournament(t *testing.T) {
	t.Run("resumes tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/resume",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.resumeCalled)
		assert.Equal(t, id, srv.resumeID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/invalid-id/resume",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.resumeCalled)
	})

	t.Run("returns conflict when tournament cannot be resumed", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			resumeErr: domain.ErrIncorrectStatus,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/resume",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.resumeCalled)
		assert.Equal(t, id, srv.resumeID)
	})
}

func TestTournamentHandler_FinishTournament(t *testing.T) {
	t.Run("finishes tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/finish",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.finishCalled)
		assert.Equal(t, id, srv.finishID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/invalid-id/finish",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.finishCalled)
	})

	t.Run("returns conflict when tournament cannot be finished", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			finishErr: domain.ErrCantFinish,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/finish",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.finishCalled)
		assert.Equal(t, id, srv.finishID)
	})
}

func TestTournamentHandler_LevelUpTournament(t *testing.T) {
	t.Run("levels up tournament", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/level-up",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.levelUpCalled)
		assert.Equal(t, id, srv.levelUpID)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when id is invalid", func(t *testing.T) {
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/invalid-id/level-up",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.levelUpCalled)
	})

	t.Run("returns conflict when tournament cannot level up", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			levelUpErr: domain.ErrMaxBlindLevel,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(
			http.MethodPost,
			"/tournaments/"+id.String()+"/level-up",
			nil,
		)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.levelUpCalled)
		assert.Equal(t, id, srv.levelUpID)
	})
}

func TestTournamentHandler_AddRebuy(t *testing.T) {
	t.Run("adds rebuy", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/rebuy",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.addRebuyCalled)
		assert.Equal(t, id, srv.addRebuyID)
		assert.Equal(t, "A", srv.addRebuyPlayerName)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when json is invalid", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/rebuy", strings.NewReader(`{`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.addRebuyCalled)
	})

	t.Run("returns bad request when player name is invalid", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			addRebuyErr: fmt.Errorf("%w: %w",
				app.ErrValidation, app.ErrInvalidPlayerName),
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/rebuy",
			strings.NewReader(`{"player_name":" "}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.True(t, srv.addRebuyCalled)
	})

	t.Run("returns not found when player does not exist", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			addRebuyErr: domain.ErrPlayerNotFound,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/rebuy",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.True(t, srv.addRebuyCalled)
		assert.Equal(t, id, srv.addRebuyID)
	})

	t.Run("returns conflict when rebuy is not allowed", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			addRebuyErr: domain.ErrRebuyNotAllowed,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/rebuy",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.addRebuyCalled)
		assert.Equal(t, id, srv.addRebuyID)
	})
}

func TestTournamentHandler_KnockoutPlayer(t *testing.T) {
	t.Run("knocks out player", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/knockout",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.True(t, srv.knockoutCalled)
		assert.Equal(t, id, srv.knockoutID)
		assert.Equal(t, "A", srv.knockoutPlayerName)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("returns bad request when json is invalid", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/knockout", strings.NewReader(`{`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, srv.knockoutCalled)
	})

	t.Run("returns not found when player does not exist", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			knockoutErr: domain.ErrPlayerNotFound,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/knockout",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.True(t, srv.knockoutCalled)
		assert.Equal(t, id, srv.knockoutID)
	})

	t.Run("returns conflict when player cannot be knocked out", func(t *testing.T) {
		id := uuid.New()
		srv := &mockTournamentService{
			knockoutErr: domain.ErrCantKnockout,
		}
		router := NewRouter(NewTournamentHandler(srv))

		req := httptest.NewRequest(http.MethodPost,
			"/tournaments/"+id.String()+"/knockout",
			strings.NewReader(`{"player_name":"A"}`))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.True(t, srv.knockoutCalled)
		assert.Equal(t, id, srv.knockoutID)
	})
}
