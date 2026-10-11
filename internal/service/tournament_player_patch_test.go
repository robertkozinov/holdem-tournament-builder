package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func playerPatchFixture(t *testing.T) *domain.Tournament {
	t.Helper()
	in := validSaveTournamentInput()
	in.Players[0].Rebuys = []domain.Rebuy{{ID: uuid.New()}}
	in.Players[1].Rebuys = []domain.Rebuy{{ID: uuid.New(), Paid: true}}
	tr := serviceTournamentFixture(t, in)
	tr.StartingStack = domain.StackPlan{Distribution: []domain.ChipDenomination{{Value: 25, Count: 10}}}
	tr.BlindStructure = []domain.BlindLevel{{SmallBlind: 25, BigBlind: 50, Ante: 50, Duration: 20 * time.Minute}}
	return tr
}

func TestTournamentService_PatchTournamentPlayerUsesUpdateLocked(t *testing.T) {
	original := playerPatchFixture(t)
	paid := true
	repo := &mockTournamentRepository{getTournament: original}
	got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, PatchTournamentPlayerInput{BuyInPaid: &paid})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Players[0].BuyInPaid)
	assert.True(t, repo.lockedCalled)
	assert.Equal(t, 1, repo.lockedCount)
	assert.Equal(t, original.ID, repo.lockedID)
	assert.Zero(t, repo.getCount)
	assert.Zero(t, repo.ordinaryUpdateCount)
	assert.Equal(t, 1, repo.updateCount)
	assert.Same(t, repo.getTournament, got)
}

func TestTournamentService_PatchTournamentPlayerTransactionError(t *testing.T) {
	original := playerPatchFixture(t)
	before, err := domain.NewTournament(*original)
	require.NoError(t, err)
	paid := true
	rebuys := []domain.Rebuy{{ID: uuid.New(), Paid: true}}
	repoErr := errors.New("commit err")
	repo := &mockTournamentRepository{getTournament: original, lockedErr: repoErr}
	got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, PatchTournamentPlayerInput{BuyInPaid: &paid, Rebuys: &rebuys})
	require.ErrorIs(t, err, repoErr)
	assert.Nil(t, got)
	assert.True(t, repo.lockedCalled)
	assert.Equal(t, 1, repo.lockedCount)
	assert.Equal(t, original.ID, repo.lockedID)
	assert.Zero(t, repo.getCount)
	assert.Zero(t, repo.ordinaryUpdateCount)
	assert.Equal(t, 1, repo.updateCount)
	assert.Same(t, original, repo.getTournament)
	assert.Equal(t, before, original)
}

func TestTournamentService_PatchTournamentPlayer(t *testing.T) {
	tests := []struct {
		name  string
		input func(*domain.Tournament) PatchTournamentPlayerInput
	}{
		{"marks buy in paid", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			paid := true
			return PatchTournamentPlayerInput{BuyInPaid: &paid}
		}},
		{"unmarks buy in paid", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			tr.Players[0].BuyInPaid = true
			paid := false
			return PatchTournamentPlayerInput{BuyInPaid: &paid}
		}},
		{"replaces rebuys", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			rebuys := []domain.Rebuy{{ID: tr.Players[0].Rebuys[0].ID, Paid: true}, {ID: uuid.New()}}
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}},
		{"clears rebuys", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			rebuys := []domain.Rebuy{}
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}},
		{"updates both fields", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			paid := true
			rebuys := []domain.Rebuy{{ID: uuid.New(), Paid: true}}
			return PatchTournamentPlayerInput{BuyInPaid: &paid, Rebuys: &rebuys}
		}},
		{"preserves results when only payment changes", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			place := 1
			tr.Results = []domain.Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: tr.Pot()}}
			paid := true
			return PatchTournamentPlayerInput{BuyInPaid: &paid}
		}},
		{"pays rebuy without changing pot or results", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			place := 1
			tr.Results = []domain.Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: tr.Pot()}}
			rebuys := slices.Clone(tr.Players[0].Rebuys)
			rebuys[0].Paid = true
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := playerPatchFixture(t)
			input := tt.input(original)
			before, err := domain.NewTournament(*original)
			require.NoError(t, err)
			expected, err := domain.NewTournament(*original)
			require.NoError(t, err)
			if input.BuyInPaid != nil {
				expected.Players[0].BuyInPaid = *input.BuyInPaid
			}
			if input.Rebuys != nil {
				expected.Players[0].Rebuys = slices.Clone(*input.Rebuys)
			}
			repo := &mockTournamentRepository{getTournament: original}
			got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, input)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, expected, got)
			assert.Equal(t, expected.Pot(), got.Pot())
			assert.Equal(t, original.ID, repo.getID)
			assert.Equal(t, 1, repo.updateCount)
			assert.Same(t, got, repo.updatedTournament)
			assert.True(t, repo.lockedCalled)
			assert.Equal(t, 1, repo.lockedCount)
			assert.Equal(t, original.ID, repo.lockedID)
			assert.Zero(t, repo.getCount)
			assert.Zero(t, repo.ordinaryUpdateCount)
			assert.Equal(t, before, original)
			if input.Rebuys != nil && len(*input.Rebuys) > 0 {
				got.Players[0].Rebuys[0].Paid = !got.Players[0].Rebuys[0].Paid
				assert.Equal(t, expected.Players[0].Rebuys, *input.Rebuys)
			}
		})
	}
	t.Run("updates selected player rather than first player", func(t *testing.T) {
		original := playerPatchFixture(t)
		original.SettlementMode = domain.SettlementBank
		before, err := domain.NewTournament(*original)
		require.NoError(t, err)
		index := len(original.Players) - 1
		paid := true
		repo := &mockTournamentRepository{getTournament: original}
		got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[index].ID, PatchTournamentPlayerInput{BuyInPaid: &paid})
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.Players[index].BuyInPaid)
		assert.Equal(t, before.Players[:index], got.Players[:index])
		assert.Equal(t, before.Pot(), got.Pot())
		assert.Equal(t, before, original)
		assert.Equal(t, 1, repo.updateCount)
		assert.Equal(t, 1, repo.lockedCount)
		assert.Equal(t, original.ID, repo.lockedID)
		assert.Zero(t, repo.getCount)
		assert.Zero(t, repo.ordinaryUpdateCount)
	})
	t.Run("repeated patch does not duplicate rebuys", func(t *testing.T) {
		original := playerPatchFixture(t)
		rebuys := []domain.Rebuy{{ID: uuid.New()}, {ID: uuid.New(), Paid: true}}
		input := PatchTournamentPlayerInput{Rebuys: &rebuys}
		repo := &mockTournamentRepository{getTournament: original}
		srv := NewTournamentService(repo)
		first, err := srv.PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, input)
		require.NoError(t, err)
		second, err := srv.PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, input)
		require.NoError(t, err)
		assert.Equal(t, first, second)
		assert.Len(t, second.Players[0].Rebuys, 2)
		assert.Equal(t, 2, repo.updateCount)
		assert.Equal(t, 2, repo.lockedCount)
		assert.Equal(t, original.ID, repo.lockedID)
		assert.Zero(t, repo.getCount)
		assert.Zero(t, repo.ordinaryUpdateCount)
	})
}

func TestTournamentService_PatchTournamentPlayerErrors(t *testing.T) {
	paid := true
	for _, tt := range []struct {
		name         string
		tournamentID uuid.UUID
		playerID     uuid.UUID
		input        PatchTournamentPlayerInput
		wantErr      error
	}{
		{"invalid tournament id", uuid.Nil, uuid.New(), PatchTournamentPlayerInput{BuyInPaid: &paid}, app.ErrInvalidTournamentID},
		{"invalid player id", testTournamentID, uuid.Nil, PatchTournamentPlayerInput{BuyInPaid: &paid}, domain.ErrInvalidPlayerID},
		{"empty patch", testTournamentID, uuid.New(), PatchTournamentPlayerInput{}, app.ErrValidation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockTournamentRepository{}
			got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), tt.tournamentID, tt.playerID, tt.input)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, got)
			assert.False(t, repo.getCalled)
			assert.False(t, repo.updateCalled)
			assert.False(t, repo.lockedCalled)
			assert.Zero(t, repo.lockedCount)
			assert.Zero(t, repo.getCount)
			assert.Zero(t, repo.ordinaryUpdateCount)
			assert.Zero(t, repo.updateCount)
		})
	}
	t.Run("player not in this tournament", func(t *testing.T) {
		original := playerPatchFixture(t)
		before, err := domain.NewTournament(*original)
		require.NoError(t, err)
		repo := &mockTournamentRepository{getTournament: original}
		got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, uuid.New(), PatchTournamentPlayerInput{BuyInPaid: &paid})
		require.ErrorIs(t, err, domain.ErrPlayerNotFound)
		assert.NotErrorIs(t, err, app.ErrValidation)
		assert.Nil(t, got)
		assert.False(t, repo.updateCalled)
		assert.Equal(t, 1, repo.lockedCount)
		assert.Equal(t, original.ID, repo.lockedID)
		assert.Zero(t, repo.getCount)
		assert.Zero(t, repo.ordinaryUpdateCount)
		assert.Zero(t, repo.updateCount)
		assert.Nil(t, repo.updatedTournament)
		assert.Same(t, original, repo.getTournament)
		assert.Equal(t, before, original)
	})
	for _, tt := range []struct {
		name    string
		input   func(*domain.Tournament) PatchTournamentPlayerInput
		wantErr error
	}{
		{"missing rebuy id", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			rebuys := []domain.Rebuy{{}}
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}, domain.ErrInvalidRebuyID},
		{"duplicate rebuy id", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			id := uuid.New()
			rebuys := []domain.Rebuy{{ID: id}, {ID: id}}
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}, domain.ErrDuplicateRebuy},
		{"rebuy id used by another player", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			rebuys := slices.Clone(tr.Players[1].Rebuys)
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}, domain.ErrDuplicateRebuy},
		{"rebuy change invalidates saved prizes", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			place := 1
			tr.Results = []domain.Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: tr.Pot()}}
			rebuys := []domain.Rebuy{}
			return PatchTournamentPlayerInput{BuyInPaid: &paid, Rebuys: &rebuys}
		}, domain.ErrPrizeSumMismatch},
		{"pot overflow", func(tr *domain.Tournament) PatchTournamentPlayerInput {
			tr.BuyInAmount = math.MaxInt64 / 6
			rebuys := []domain.Rebuy{{ID: uuid.New()}, {ID: uuid.New()}}
			return PatchTournamentPlayerInput{Rebuys: &rebuys}
		}, domain.ErrPotOverflow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := playerPatchFixture(t)
			input := tt.input(original)
			before, err := domain.NewTournament(*original)
			require.NoError(t, err)
			repo := &mockTournamentRepository{getTournament: original}
			got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, input)
			require.ErrorIs(t, err, app.ErrValidation)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, got)
			assert.False(t, repo.updateCalled)
			assert.Equal(t, 1, repo.lockedCount)
			assert.Equal(t, original.ID, repo.lockedID)
			assert.Zero(t, repo.getCount)
			assert.Zero(t, repo.ordinaryUpdateCount)
			assert.Zero(t, repo.updateCount)
			assert.Nil(t, repo.updatedTournament)
			assert.Same(t, original, repo.getTournament)
			assert.Equal(t, before, original)
		})
	}
	for _, repoErr := range []error{app.ErrTournamentNotFound, errors.New("get err")} {
		t.Run("read failure", func(t *testing.T) {
			repo := &mockTournamentRepository{getErr: repoErr}
			got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), testTournamentID, uuid.New(), PatchTournamentPlayerInput{BuyInPaid: &paid})
			require.ErrorIs(t, err, repoErr)
			assert.Nil(t, got)
			assert.False(t, repo.updateCalled)
			assert.Equal(t, 1, repo.lockedCount)
			assert.Equal(t, testTournamentID, repo.lockedID)
			assert.Zero(t, repo.getCount)
			assert.Zero(t, repo.ordinaryUpdateCount)
			assert.Zero(t, repo.updateCount)
			assert.Nil(t, repo.updatedTournament)
		})
	}
	t.Run("write failure preserves loaded card", func(t *testing.T) {
		original := playerPatchFixture(t)
		before, err := domain.NewTournament(*original)
		require.NoError(t, err)
		repoErr := errors.New("update err")
		repo := &mockTournamentRepository{getTournament: original, updateErr: repoErr}
		rebuys := []domain.Rebuy{{ID: uuid.New(), Paid: true}}
		got, err := NewTournamentService(repo).PatchTournamentPlayer(context.Background(), original.ID, original.Players[0].ID, PatchTournamentPlayerInput{BuyInPaid: &paid, Rebuys: &rebuys})
		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
		assert.Equal(t, 1, repo.updateCount)
		assert.Equal(t, 1, repo.lockedCount)
		assert.Equal(t, original.ID, repo.lockedID)
		assert.Zero(t, repo.getCount)
		assert.Zero(t, repo.ordinaryUpdateCount)
		assert.Same(t, original, repo.getTournament)
		assert.Equal(t, before, original)
	})
}
