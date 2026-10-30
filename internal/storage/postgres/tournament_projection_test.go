package postgres

import (
	"context"
	"testing"

	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentPlayerProjections(t *testing.T) {
	winnerID := uuid.New()
	loserID := uuid.New()
	card := &domain.Tournament{
		BuyInAmount: 1000,
		Players: []domain.TournamentPlayer{
			{
				ID:        winnerID,
				BuyInPaid: false,
				Rebuys:    []domain.Rebuy{{ID: uuid.New(), Paid: false}},
			},
			{ID: loserID, BuyInPaid: true},
		},
		Results: []domain.Result{{PlayerID: winnerID, Prize: 3000}},
	}

	assert.Equal(t, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 2000, prize: 3000, included: true},
		{playerID: loserID, contribution: 1000, prize: 0, included: true},
	}, tournamentPlayerProjections(card))

	card.Results = nil
	assert.Equal(t, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 2000, prize: 0, included: false},
		{playerID: loserID, contribution: 1000, prize: 0, included: false},
	}, tournamentPlayerProjections(card))
}

func TestSyncTournamentPlayersWritesProjections(t *testing.T) {
	ownerID := uuid.New()
	winnerID := uuid.New()
	loserID := uuid.New()
	card := &domain.Tournament{
		ID:          uuid.New(),
		BuyInAmount: 1000,
		Players: []domain.TournamentPlayer{
			{ID: winnerID, Rebuys: []domain.Rebuy{{ID: uuid.New()}}},
			{ID: loserID},
		},
		Results: []domain.Result{{PlayerID: winnerID, Prize: 3000}},
	}

	var arguments [][]any
	tx := &stubTournamentTx{exec: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
		arguments = append(arguments, append([]any(nil), args...))
		return pgconn.NewCommandTag("INSERT 0 2"), nil
	}}

	require.NoError(t, syncTournamentPlayers(context.Background(), tx, ownerID, card))
	require.Len(t, arguments, 2)
	assert.Equal(t, []any{card.ID, ownerID}, arguments[0])
	assert.Equal(t, []any{card.ID, []uuid.UUID{winnerID, loserID}, []int64{2000, 1000}, []int64{3000, 0}, ownerID, true}, arguments[1])
}
