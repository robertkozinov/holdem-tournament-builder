package service

import (
	"context"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTournamentRepository struct {
	createCalled      bool
	createdTournament *domain.Tournament
	createID          int64
	createErr         error

	getCalled     bool
	getID         int64
	getTournament *domain.Tournament
	getErr        error

	deleteCalled bool
	deleteID     int64
	deleteErr    error

	updateCalled      bool
	updatedTournament *domain.Tournament
	updateErr         error
}

func (r *mockTournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) (int64, error) {
	r.createCalled = true
	r.createdTournament = tournament

	if r.createErr != nil {
		return 0, r.createErr
	}

	return r.createID, nil
}

func (r *mockTournamentRepository) GetByID(ctx context.Context, id int64) (*domain.Tournament, error) {
	r.getCalled = true
	r.getID = id

	if r.getErr != nil {
		return nil, r.getErr
	}

	return r.getTournament, nil
}

func (r *mockTournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	r.updateCalled = true
	r.updatedTournament = tournament

	if r.updateErr != nil {
		return r.updateErr
	}

	return nil
}

func (r *mockTournamentRepository) Delete(ctx context.Context, id int64) error {
	r.deleteCalled = true
	r.deleteID = id

	if r.deleteErr != nil {
		return r.deleteErr
	}

	return nil
}

type createTournamentInput struct {
	name        string
	date        time.Time
	players     []string
	buyInAmount int64
	chips       []domain.ChipDenomination
	rebuyRules  domain.RebuyRules
	duration    time.Duration
	stack       domain.StackPlan
	blinds      []domain.BlindLevel
	payouts     []domain.PayoutSpot
}

func validCreateTournamentInput() createTournamentInput {
	return createTournamentInput{
		name:        "Friday Game",
		date:        time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC),
		players:     []string{"A", "B", "C", "D"},
		buyInAmount: 1000,
		chips:       []domain.ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
		rebuyRules:  domain.RebuyRules{Allowed: true, MaxLevel: 3},
		duration:    2 * time.Hour,
		stack:       domain.StackPlan{},
		blinds:      nil,
		payouts:     nil,
	}
}

func createTournament(service *TournamentService, input createTournamentInput) (int64, error) {
	return service.CreateTournament(
		context.Background(),
		input.name,
		input.date,
		input.players,
		input.buyInAmount,
		input.chips,
		input.rebuyRules,
		input.duration,
		input.stack,
		input.blinds,
		input.payouts,
	)
}

func tournamentWithStatus(status domain.Status) *domain.Tournament {
	return &domain.Tournament{
		ID:     10,
		Name:   "Friday Game",
		Status: status,
	}
}

func TestTournamentService_CreateTournament(t *testing.T) {
	t.Run("creates tournament", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: 10}
		service := NewTournamentService(repo)

		id, err := createTournament(service, validCreateTournamentInput())

		require.NoError(t, err)
		assert.Equal(t, int64(10), id)
		assert.True(t, repo.createCalled)
		require.NotNil(t, repo.createdTournament)
		assert.Equal(t, "Friday Game", repo.createdTournament.Name)
	})

	t.Run("returns error when tournament is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: 10}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.name = ""

		id, err := createTournament(service, input)

		assert.Equal(t, int64(0), id)
		assert.ErrorIs(t, err, domain.ErrEmptyName)
		assert.False(t, repo.createCalled)
	})

	t.Run("returns error when repository create fails", func(t *testing.T) {
		repoErr := errors.New("repo error")
		repo := &mockTournamentRepository{
			createID:  10,
			createErr: repoErr,
		}
		service := NewTournamentService(repo)

		id, err := createTournament(service, validCreateTournamentInput())

		assert.Equal(t, int64(0), id)
		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.createCalled)
	})
}

func TestTournamentService_GetTournamentByID(t *testing.T) {
	t.Run("gets tournament", func(t *testing.T) {
		wantTr := &domain.Tournament{ID: 10, Name: "Friday game"}
		repo := &mockTournamentRepository{getTournament: wantTr}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), 10)

		require.NoError(t, err)
		assert.Same(t, wantTr, got)
		assert.True(t, repo.getCalled)
		assert.Equal(t, int64(10), repo.getID)
	})

	t.Run("returns error when id is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), -1)

		require.Error(t, err)
		assert.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, got)
		assert.False(t, repo.getCalled)
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repoErr := app.ErrTournamentNotFound
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), 10)

		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
		assert.True(t, repo.getCalled)
	})
}

func TestTournamentService_DeleteTournament(t *testing.T) {
	t.Run("deletes tournament", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), 10)

		require.NoError(t, err)
		assert.True(t, repo.deleteCalled)
		assert.Equal(t, int64(10), repo.deleteID)
	})
	t.Run("returns error when id is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), -1)

		assert.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.False(t, repo.deleteCalled)
	})
	t.Run("returns error when repo fails", func(t *testing.T) {
		repoErr := app.ErrTournamentNotFound
		repo := &mockTournamentRepository{deleteErr: repoErr}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), 10)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.deleteCalled)
		assert.Equal(t, int64(10), repo.deleteID)
	})
}

func TestTournamentService_StartTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("starts tournament", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusCreated)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), 10, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, int64(10), repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusRunning, repo.updatedTournament.Status)
		assert.Equal(t, now, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be started", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusRunning)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := tournamentWithStatus(domain.StatusCreated)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), 10, now)

		require.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func TestTournamentService_PauseTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("pauses tournament", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusRunning)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), 10, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, int64(10), repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusPaused, repo.updatedTournament.Status)
		assert.Equal(t, now, repo.updatedTournament.PausedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be paused", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusPaused)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := tournamentWithStatus(domain.StatusRunning)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), 10, now)

		require.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func TestTournamentService_ResumeTournament(t *testing.T) {
	levelStartedAt := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)
	pausedAt := levelStartedAt.Add(10 * time.Minute)
	now := pausedAt.Add(5 * time.Minute)
	wantLevelStartedAt := levelStartedAt.Add(5 * time.Minute)

	t.Run("resumes tournament", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusPaused)
		tr.LevelStartedAt = levelStartedAt
		tr.PausedAt = pausedAt

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), 10, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, int64(10), repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusRunning, repo.updatedTournament.Status)
		assert.Equal(t, wantLevelStartedAt, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be resumed", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusRunning)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")

		tr := tournamentWithStatus(domain.StatusPaused)
		tr.LevelStartedAt = levelStartedAt
		tr.PausedAt = pausedAt

		repo := &mockTournamentRepository{
			getTournament: tr,
			updateErr:     repoErr,
		}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), 10, now)

		require.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithBlinds() *domain.Tournament {
	return &domain.Tournament{
		ID:           10,
		Name:         "Friday Game",
		Status:       domain.StatusRunning,
		CurrentLevel: 0,
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Duration: 10 * time.Minute},
		},
	}
}

func TestTournamentService_LevelUpTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("levels up tournament", func(t *testing.T) {
		tr := runningTournamentWithBlinds()
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), 10, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, int64(10), repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, 1, repo.updatedTournament.CurrentLevel)
		assert.Equal(t, now, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant level up", func(t *testing.T) {
		tr := runningTournamentWithBlinds()
		tr.CurrentLevel = 1

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, domain.ErrMaxBlindLevel)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithBlinds()
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), 10, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithRebuy() *domain.Tournament {
	return &domain.Tournament{
		ID:          10,
		Name:        "Friday Game",
		Status:      domain.StatusRunning,
		Players:     []string{"A", "B"},
		BuyInAmount: 1000,
		RebuyRules:  domain.RebuyRules{Allowed: true, MaxLevel: 3},
		Contributions: map[string]int64{
			"A": 1000,
			"B": 1000,
		},
	}
}

func TestTournamentService_AddRebuy(t *testing.T) {
	t.Run("adds rebuy", func(t *testing.T) {
		tr := runningTournamentWithRebuy()
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), 10, "A")

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, int64(2000), repo.updatedTournament.Contributions["A"])
	})

	t.Run("returns error when player name is empty", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), 10, "   ")

		assert.ErrorIs(t, err, app.ErrInvalidPlayerName)
		assert.False(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), 10, "A")

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when rebuy cant be added", func(t *testing.T) {
		tr := runningTournamentWithRebuy()
		tr.Status = domain.StatusCreated

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), 10, "A")

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithRebuy()
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), 10, "A")

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithPlayers() *domain.Tournament {
	return &domain.Tournament{
		ID:          10,
		Name:        "Friday Game",
		Status:      domain.StatusRunning,
		Players:     []string{"A", "B", "C"},
		BuyInAmount: 1000,
		Contributions: map[string]int64{
			"A": 1000,
			"B": 1000,
			"C": 1000,
		},
		PayoutSpots: []domain.PayoutSpot{
			{Place: 1, Kind: domain.PayoutRemainder},
		},
	}
}

func TestTournamentService_KnockoutPlayer(t *testing.T) {
	t.Run("knocks out player", func(t *testing.T) {
		tr := runningTournamentWithPlayers()
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), 10, "B")

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.NotContains(t, repo.updatedTournament.Players, "B")
		require.Len(t, repo.updatedTournament.Results, 1)
		assert.Equal(t, "B", repo.updatedTournament.Results[0].Name)
		assert.Equal(t, 3, repo.updatedTournament.Results[0].Place)
	})

	t.Run("returns error when player name is empty", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), 10, "   ")

		assert.ErrorIs(t, err, app.ErrInvalidPlayerName)
		assert.False(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), 10, "B")

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when player cant be knocked out", func(t *testing.T) {
		tr := runningTournamentWithPlayers()
		tr.Status = domain.StatusCreated

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), 10, "B")

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithPlayers()
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), 10, "B")

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentReadyToFinish() *domain.Tournament {
	return &domain.Tournament{
		ID:          10,
		Name:        "Friday Game",
		Status:      domain.StatusRunning,
		Players:     []string{"A"},
		BuyInAmount: 1000,
		Contributions: map[string]int64{
			"A": 1000,
			"B": 1000,
		},
		PayoutSpots: []domain.PayoutSpot{
			{Place: 1, Kind: domain.PayoutRemainder},
		},
	}
}

func TestTournamentService_FinishTournament(t *testing.T) {
	t.Run("finishes tournament", func(t *testing.T) {
		tr := runningTournamentReadyToFinish()
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), 10)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusFinished, repo.updatedTournament.Status)
		require.Len(t, repo.updatedTournament.Results, 1)
		assert.Equal(t, "A", repo.updatedTournament.Results[0].Name)
		assert.Equal(t, 1, repo.updatedTournament.Results[0].Place)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), 10)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be finished", func(t *testing.T) {
		tr := runningTournamentReadyToFinish()
		tr.Players = []string{"A", "B"}

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), 10)

		assert.ErrorIs(t, err, domain.ErrCantFinish)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentReadyToFinish()
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), 10)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}
