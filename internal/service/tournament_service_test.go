package service

import (
	"context"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTournamentID = uuid.MustParse("00000000-0000-0000-0000-000000000010")

type mockTournamentRepository struct {
	createCalled      bool
	createdTournament *domain.Tournament
	createID          uuid.UUID
	createErr         error

	getCalled     bool
	getID         uuid.UUID
	getTournament *domain.Tournament
	getErr        error

	deleteCalled bool
	deleteID     uuid.UUID
	deleteErr    error

	updateCalled      bool
	updateCalls       int
	updatedTournament *domain.Tournament
	updateErr         error
}

func (r *mockTournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) (uuid.UUID, error) {
	r.createCalled = true
	r.createdTournament = tournament

	if r.createErr != nil {
		return uuid.Nil, r.createErr
	}

	return r.createID, nil
}

func (r *mockTournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	r.getCalled = true
	r.getID = id

	if r.getErr != nil {
		return nil, r.getErr
	}

	return r.getTournament, nil
}

func (r *mockTournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	r.updateCalled = true
	r.updateCalls++
	r.updatedTournament = tournament

	if r.updateErr != nil {
		return r.updateErr
	}

	return nil
}

func (r *mockTournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.deleteCalled = true
	r.deleteID = id

	if r.deleteErr != nil {
		return r.deleteErr
	}

	return nil
}

func validCreateTournamentInput() CreateTournamentInput {
	return CreateTournamentInput{
		Name:              "Friday Game",
		Date:              time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC),
		Players:           []string{"A", "B", "C", "D"},
		BuyInAmount:       1000,
		Chips:             []domain.ChipDenomination{{Value: 25, Count: 80}, {Value: 100, Count: 80}, {Value: 500, Count: 40}},
		Style:             domain.StyleStandard,
		Duration:          2 * time.Hour,
		LevelDuration:     20 * time.Minute,
		RebuyAllowed:      true,
		RebuyMaxLevel:     3,
		PayoutMode:        PayoutModeDefault,
		PayoutFixedBuyIns: nil,
	}
}

func createTournament(service *TournamentService, input CreateTournamentInput) (uuid.UUID, error) {
	return service.CreateTournament(context.Background(), input)
}

func tournamentWithStatus(status domain.Status) *domain.Tournament {
	return &domain.Tournament{
		ID:     testTournamentID,
		Name:   "Friday Game",
		Status: status,
	}
}

func runningTournament(now time.Time) *domain.Tournament {
	return &domain.Tournament{
		ID:             testTournamentID,
		Name:           "Friday Game",
		Status:         domain.StatusRunning,
		CurrentLevel:   0,
		LevelStartedAt: now,
		BlindStructure: []domain.BlindLevel{
			{Duration: 20 * time.Minute},
		},
	}
}

func TestTournamentService_CreateTournament(t *testing.T) {
	t.Run("creates tournament", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		id, err := createTournament(service, validCreateTournamentInput())

		require.NoError(t, err)
		assert.Equal(t, testTournamentID, id)
		assert.True(t, repo.createCalled)
		require.NotNil(t, repo.createdTournament)
		assert.Equal(t, "Friday Game", repo.createdTournament.Name)
		assert.Equal(t, domain.StatusCreated, repo.createdTournament.Status)
		assert.Equal(t, domain.RebuyRules{Allowed: true, MaxLevel: 3}, repo.createdTournament.RebuyRules)
		assert.NotEmpty(t, repo.createdTournament.StartingStack.Distribution)
		assert.NotEmpty(t, repo.createdTournament.BlindStructure)
		assert.NotEmpty(t, repo.createdTournament.PayoutSpots)
	})

	t.Run("returns error when tournament is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.Name = ""

		id, err := createTournament(service, input)

		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, domain.ErrEmptyName)
		assert.False(t, repo.createCalled)
	})

	t.Run("creates tournament with custom payouts", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.PayoutMode = PayoutModeCustom
		input.PayoutFixedBuyIns = []int{1}

		id, err := createTournament(service, input)

		require.NoError(t, err)
		assert.Equal(t, testTournamentID, id)
		require.NotNil(t, repo.createdTournament)
		assert.Equal(t, []domain.PayoutSpot{
			{Place: 1, Kind: domain.PayoutRemainder},
			{Place: 2, Kind: domain.PayoutFixed, BuyInsValue: 1},
		}, repo.createdTournament.PayoutSpots)
	})

	t.Run("returns error when payout mode is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.PayoutMode = PayoutMode("unknown")

		id, err := createTournament(service, input)

		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, app.ErrInvalidPayoutMode)
		assert.False(t, repo.createCalled)
	})

	t.Run("returns error when stack generation fails", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.Chips = nil

		id, err := createTournament(service, input)

		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, domain.ErrEmptyChipSet)
		assert.False(t, repo.createCalled)
	})

	t.Run("returns error when blind generation fails", func(t *testing.T) {
		repo := &mockTournamentRepository{createID: testTournamentID}
		service := NewTournamentService(repo)

		input := validCreateTournamentInput()
		input.Duration = 20 * time.Minute
		input.LevelDuration = 20 * time.Minute

		id, err := createTournament(service, input)

		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, domain.ErrNotEnoughLevels)
		assert.False(t, repo.createCalled)
	})

	t.Run("returns error when repository create fails", func(t *testing.T) {
		repoErr := errors.New("repo error")
		repo := &mockTournamentRepository{
			createID:  testTournamentID,
			createErr: repoErr,
		}
		service := NewTournamentService(repo)

		id, err := createTournament(service, validCreateTournamentInput())

		assert.Equal(t, uuid.Nil, id)
		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.createCalled)
	})
}

func TestTournamentService_GetTournamentByID(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)

	t.Run("gets tournament", func(t *testing.T) {
		wantTr := &domain.Tournament{ID: testTournamentID, Name: "Friday game"}
		repo := &mockTournamentRepository{getTournament: wantTr}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.Same(t, wantTr, got)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
		assert.Equal(t, testTournamentID, repo.getID)
	})

	t.Run("returns error when id is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), uuid.Nil, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, got)
		assert.False(t, repo.getCalled)
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repoErr := app.ErrTournamentNotFound
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		got, err := service.GetTournamentByID(context.Background(), testTournamentID, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
		assert.True(t, repo.getCalled)
	})

	t.Run("advances expired blind levels", func(t *testing.T) {
		tr := &domain.Tournament{
			ID:             testTournamentID,
			Status:         domain.StatusRunning,
			CurrentLevel:   0,
			LevelStartedAt: now.Add(-45 * time.Minute),
			BlindStructure: []domain.BlindLevel{
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
			},
		}

		repo := &mockTournamentRepository{getTournament: tr}

		service := NewTournamentService(repo)

		gotTr, err := service.GetTournamentByID(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.Equal(t, 2, gotTr.CurrentLevel)
		assert.Equal(t, now.Add(-5*time.Minute), gotTr.LevelStartedAt)
		assert.True(t, repo.updateCalled)
		assert.Equal(t, 1, repo.updateCalls)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, gotTr, repo.updatedTournament)
	})

	t.Run("does not update when blind level has not expired", func(t *testing.T) {
		tr := &domain.Tournament{
			ID:             testTournamentID,
			Status:         domain.StatusRunning,
			CurrentLevel:   0,
			LevelStartedAt: now.Add(-10 * time.Minute),
			BlindStructure: []domain.BlindLevel{
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
			},
		}

		repo := &mockTournamentRepository{getTournament: tr}

		service := NewTournamentService(repo)

		gotTr, err := service.GetTournamentByID(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.Equal(t, 0, gotTr.CurrentLevel)
		assert.Equal(t, now.Add(-10*time.Minute), gotTr.LevelStartedAt)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		tr := &domain.Tournament{
			ID:             testTournamentID,
			Status:         domain.StatusRunning,
			CurrentLevel:   0,
			LevelStartedAt: now.Add(-25 * time.Minute),
			BlindStructure: []domain.BlindLevel{
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
				{Duration: 20 * time.Minute},
			},
		}

		repoErr := errors.New("repo err")

		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}

		service := NewTournamentService(repo)

		gotTr, err := service.GetTournamentByID(context.Background(), testTournamentID, now)

		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, gotTr)

		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)

		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, 1, repo.updatedTournament.CurrentLevel)
		assert.Equal(t, now.Add(-5*time.Minute), repo.updatedTournament.LevelStartedAt)
	})
}

func TestTournamentService_DeleteTournament(t *testing.T) {
	t.Run("deletes tournament", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), testTournamentID)

		require.NoError(t, err)
		assert.True(t, repo.deleteCalled)
		assert.Equal(t, testTournamentID, repo.deleteID)
	})
	t.Run("returns error when id is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), uuid.Nil)

		assert.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.False(t, repo.deleteCalled)
	})
	t.Run("returns error when repo fails", func(t *testing.T) {
		repoErr := app.ErrTournamentNotFound
		repo := &mockTournamentRepository{deleteErr: repoErr}
		service := NewTournamentService(repo)

		err := service.DeleteTournament(context.Background(), testTournamentID)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.deleteCalled)
		assert.Equal(t, testTournamentID, repo.deleteID)
	})
}

func TestTournamentService_StartTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("starts tournament", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusCreated)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusRunning, repo.updatedTournament.Status)
		assert.Equal(t, now, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be started", func(t *testing.T) {
		tr := runningTournament(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := tournamentWithStatus(domain.StatusCreated)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.StartTournament(context.Background(), testTournamentID, now)

		require.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func TestTournamentService_PauseTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("pauses tournament", func(t *testing.T) {
		tr := runningTournament(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusPaused, repo.updatedTournament.Status)
		assert.Equal(t, now, repo.updatedTournament.PausedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be paused", func(t *testing.T) {
		tr := tournamentWithStatus(domain.StatusPaused)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournament(now)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.PauseTournament(context.Background(), testTournamentID, now)

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

		err := service.ResumeTournament(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, domain.StatusRunning, repo.updatedTournament.Status)
		assert.Equal(t, wantLevelStartedAt, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be resumed", func(t *testing.T) {
		tr := runningTournament(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.ResumeTournament(context.Background(), testTournamentID, now)

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

		err := service.ResumeTournament(context.Background(), testTournamentID, now)

		require.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithBlinds(now time.Time) *domain.Tournament {
	return &domain.Tournament{
		ID:             testTournamentID,
		Name:           "Friday Game",
		Status:         domain.StatusRunning,
		CurrentLevel:   0,
		LevelStartedAt: now,
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Duration: 10 * time.Minute},
		},
	}
}

func TestTournamentService_LevelUpTournament(t *testing.T) {
	now := time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC)

	t.Run("levels up tournament", func(t *testing.T) {
		tr := runningTournamentWithBlinds(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, 1, repo.updatedTournament.CurrentLevel)
		assert.Equal(t, now, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("does not level up twice when blind level has expired", func(t *testing.T) {
		tr := runningTournamentWithBlinds(now)
		tr.LevelStartedAt = now.Add(-10 * time.Minute)
		tr.BlindStructure = append(tr.BlindStructure, domain.BlindLevel{
			SmallBlind: 100,
			BigBlind:   200,
			Duration:   10 * time.Minute,
		})

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), testTournamentID, now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.Equal(t, 1, repo.updateCalls)
		assert.Equal(t, testTournamentID, repo.getID)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, 1, repo.updatedTournament.CurrentLevel)
		assert.Equal(t, now, repo.updatedTournament.LevelStartedAt)
	})

	t.Run("returns error when id is invalid", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), uuid.Nil, now)

		assert.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.False(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant level up", func(t *testing.T) {
		tr := runningTournamentWithBlinds(now)
		tr.CurrentLevel = 1

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, domain.ErrMaxBlindLevel)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithBlinds(now)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.LevelUpTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithRebuy(now time.Time) *domain.Tournament {
	return &domain.Tournament{
		ID:             testTournamentID,
		Name:           "Friday Game",
		Status:         domain.StatusRunning,
		Players:        []string{"A", "B"},
		BuyInAmount:    1000,
		LevelStartedAt: now,
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Duration: 10 * time.Minute},
		},
		RebuyRules: domain.RebuyRules{Allowed: true, MaxLevel: 3},
		Contributions: map[string]int64{
			"A": 1000,
			"B": 1000,
		},
	}
}

func TestTournamentService_AddRebuy(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	t.Run("adds rebuy", func(t *testing.T) {
		tr := runningTournamentWithRebuy(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), testTournamentID, "A", now)

		require.NoError(t, err)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
		require.NotNil(t, repo.updatedTournament)
		assert.Equal(t, int64(2000), repo.updatedTournament.Contributions["A"])
	})

	t.Run("returns error when player name is empty", func(t *testing.T) {
		repo := &mockTournamentRepository{}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), testTournamentID, "   ", now)

		assert.ErrorIs(t, err, app.ErrInvalidPlayerName)
		assert.False(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), testTournamentID, "A", now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when rebuy cant be added", func(t *testing.T) {
		tr := runningTournamentWithRebuy(now)
		tr.Status = domain.StatusCreated

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), testTournamentID, "A", now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithRebuy(now)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.AddRebuy(context.Background(), testTournamentID, "A", now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentWithPlayers(now time.Time) *domain.Tournament {
	return &domain.Tournament{
		ID:             testTournamentID,
		Name:           "Friday Game",
		Status:         domain.StatusRunning,
		Players:        []string{"A", "B", "C"},
		BuyInAmount:    1000,
		LevelStartedAt: now,
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Duration: 10 * time.Minute},
		},
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
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	t.Run("knocks out player", func(t *testing.T) {
		tr := runningTournamentWithPlayers(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), testTournamentID, "B", now)

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

		err := service.KnockoutPlayer(context.Background(), testTournamentID, "   ", now)

		assert.ErrorIs(t, err, app.ErrInvalidPlayerName)
		assert.False(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when get tournament fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		repo := &mockTournamentRepository{getErr: repoErr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), testTournamentID, "B", now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when player cant be knocked out", func(t *testing.T) {
		tr := runningTournamentWithPlayers(now)
		tr.Status = domain.StatusCreated

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), testTournamentID, "B", now)

		assert.ErrorIs(t, err, domain.ErrIncorrectStatus)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentWithPlayers(now)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.KnockoutPlayer(context.Background(), testTournamentID, "B", now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}

func runningTournamentReadyToFinish(now time.Time) *domain.Tournament {
	return &domain.Tournament{
		ID:             testTournamentID,
		Name:           "Friday Game",
		Status:         domain.StatusRunning,
		Players:        []string{"A"},
		BuyInAmount:    1000,
		LevelStartedAt: now,
		BlindStructure: []domain.BlindLevel{
			{SmallBlind: 25, BigBlind: 50, Duration: 10 * time.Minute},
			{SmallBlind: 50, BigBlind: 100, Duration: 10 * time.Minute},
		},
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
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)

	t.Run("finishes tournament", func(t *testing.T) {
		tr := runningTournamentReadyToFinish(now)
		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), testTournamentID, now)

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

		err := service.FinishTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when tournament cant be finished", func(t *testing.T) {
		tr := runningTournamentReadyToFinish(now)
		tr.Players = []string{"A", "B"}

		repo := &mockTournamentRepository{getTournament: tr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, domain.ErrCantFinish)
		assert.True(t, repo.getCalled)
		assert.False(t, repo.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		repoErr := errors.New("repo err")
		tr := runningTournamentReadyToFinish(now)
		repo := &mockTournamentRepository{getTournament: tr, updateErr: repoErr}
		service := NewTournamentService(repo)

		err := service.FinishTournament(context.Background(), testTournamentID, now)

		assert.ErrorIs(t, err, repoErr)
		assert.True(t, repo.getCalled)
		assert.True(t, repo.updateCalled)
	})
}
