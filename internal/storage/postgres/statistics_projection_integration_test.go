package postgres

import (
	"slices"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentStatisticsProjectionIntegration(t *testing.T) {
	ctx, pool, _, ownerID := setupPlayerRepositoryTest(t)
	repo := NewTournamentRepository(pool)
	assertProjection := func(tournamentID uuid.UUID, expected []tournamentPlayerProjection) {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT player_id, contribution, prize, included_in_statistics
			FROM tournament_players WHERE tournament_id = $1 AND user_id = $2`, tournamentID, ownerID)
		require.NoError(t, err)
		defer rows.Close()

		actual := make([]tournamentPlayerProjection, 0, len(expected))
		for rows.Next() {
			var projection tournamentPlayerProjection
			require.NoError(t, rows.Scan(&projection.playerID, &projection.contribution, &projection.prize, &projection.included))
			actual = append(actual, projection)
		}
		require.NoError(t, rows.Err())
		require.Len(t, actual, len(expected))
		assert.ElementsMatch(t, expected, actual)
	}

	build := registryTournamentBuilder(validTournament(t))
	card, err := repo.SaveWithPlayers(ctx, ownerID, uuid.Nil, []domain.TournamentPlayer{
		{Name: "Projection Winner", BuyInPaid: false},
		{Name: "Projection Loser", BuyInPaid: true},
	}, build)
	require.NoError(t, err)
	winnerID := card.Players[0].ID
	loserID := card.Players[1].ID
	assertProjection(card.ID, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 1000, included: false},
		{playerID: loserID, contribution: 1000, included: false},
	})

	tournamentService := service.NewTournamentService(repo)
	patchCtx := app.WithUserID(ctx, ownerID)
	rebuys := []domain.Rebuy{{ID: uuid.New(), Paid: false}}
	card, err = tournamentService.PatchTournamentPlayer(patchCtx, card.ID, winnerID, service.PatchTournamentPlayerInput{Rebuys: &rebuys})
	require.NoError(t, err)
	assertProjection(card.ID, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 2000, included: false},
		{playerID: loserID, contribution: 1000, included: false},
	})

	noRebuys := []domain.Rebuy{}
	card, err = tournamentService.PatchTournamentPlayer(patchCtx, card.ID, winnerID, service.PatchTournamentPlayerInput{Rebuys: &noRebuys})
	require.NoError(t, err)
	assertProjection(card.ID, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 1000, included: false},
		{playerID: loserID, contribution: 1000, included: false},
	})

	card, err = tournamentService.PatchTournamentPlayer(patchCtx, card.ID, winnerID, service.PatchTournamentPlayerInput{Rebuys: &rebuys})
	require.NoError(t, err)
	firstPlace := 1
	card.Results = []domain.Result{{PlayerID: winnerID, Place: &firstPlace, Prize: 3000}}
	require.NoError(t, repo.Update(ctx, ownerID, card))
	completed := []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 2000, prize: 3000, included: true},
		{playerID: loserID, contribution: 1000, prize: 0, included: true},
	}
	assertProjection(card.ID, completed)

	require.NoError(t, repo.Update(ctx, ownerID, card))
	assertProjection(card.ID, completed)

	rejected := *card
	rejected.Players = slices.Clone(card.Players)
	rejected.Players[0].Rebuys = append(slices.Clone(card.Players[0].Rebuys), domain.Rebuy{ID: uuid.New()})
	require.ErrorIs(t, repo.Update(ctx, ownerID, &rejected), domain.ErrPrizeSumMismatch)
	assertProjection(card.ID, completed)

	card.Results = nil
	require.NoError(t, repo.Update(ctx, ownerID, card))
	assertProjection(card.ID, []tournamentPlayerProjection{
		{playerID: winnerID, contribution: 2000, included: false},
		{playerID: loserID, contribution: 1000, included: false},
	})

	require.NoError(t, repo.Delete(ctx, ownerID, card.ID))
	assertProjection(card.ID, nil)
}
