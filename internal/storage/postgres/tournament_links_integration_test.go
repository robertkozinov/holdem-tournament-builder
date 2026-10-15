package postgres

import (
	"errors"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentLinksMatchCardIntegration(t *testing.T) {
	ctx, pool, _, owner := setupPlayerRepositoryTest(t)
	repo := NewTournamentRepository(pool)
	build := registryTournamentBuilder(validTournament(t))
	card, err := repo.SaveWithPlayers(ctx, owner, uuid.Nil, []domain.TournamentPlayer{{Name: "Link A"}, {Name: "Link B"}}, build)
	require.NoError(t, err)
	check := func(expected *domain.Tournament) {
		t.Helper()
		rows, err := pool.Query(ctx, "SELECT player_id FROM tournament_players WHERE tournament_id=$1 AND user_id=$2", expected.ID, owner)
		require.NoError(t, err)
		ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		require.NoError(t, err)
		want := make([]uuid.UUID, 0, len(expected.Players))
		for _, p := range expected.Players {
			want = append(want, p.ID)
		}
		assert.ElementsMatch(t, want, ids)
		stored, err := repo.GetByID(ctx, owner, expected.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, tournamentPlayerIDs([]*domain.Tournament{expected}), tournamentPlayerIDs([]*domain.Tournament{stored}))
	}
	check(card)
	paid := true
	card, err = service.NewTournamentService(repo).PatchTournamentPlayer(app.WithUserID(ctx, owner), card.ID, card.Players[0].ID, service.PatchTournamentPlayerInput{BuyInPaid: &paid})
	require.NoError(t, err)
	check(card)
	card, err = repo.SaveWithPlayers(ctx, owner, card.ID, []domain.TournamentPlayer{{ID: card.Players[1].ID}, {Name: "Link C"}}, build)
	require.NoError(t, err)
	check(card)
	cause := errors.New("reject replacement")
	_, err = repo.SaveWithPlayers(ctx, owner, card.ID, []domain.TournamentPlayer{{ID: card.Players[0].ID}, {Name: "Link D"}}, func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error) { return nil, cause })
	require.ErrorIs(t, err, cause)
	check(card)
}
