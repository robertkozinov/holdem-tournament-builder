package postgres

import (
	"context"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentRepository_FiltersOwner(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	db := &stubTournamentDB{queryRow: func(ctx context.Context, query string, args ...any) pgx.Row {
		assert.Contains(t, compactSQL(query), "user_id = $2")
		assert.Equal(t, []any{id, owner}, args)
		return tournamentRowFunc(func(...any) error { return pgx.ErrNoRows })
	}}
	_, err := (&TournamentRepository{pool: db}).GetByID(context.Background(), owner, id)
	require.ErrorIs(t, err, app.ErrTournamentNotFound)
}

func TestTournamentRepository_RejectsNilOwner(t *testing.T) {
	repo := NewTournamentRepository(nil)
	ctx := context.Background()
	id := uuid.New()
	t.Run("create", func(t *testing.T) {
		_, err := repo.Create(ctx, uuid.Nil, validTournament(t))
		require.ErrorIs(t, err, app.ErrUnauthenticated)
	})
	t.Run("get", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.Nil, id)
		require.ErrorIs(t, err, app.ErrUnauthenticated)
	})
	t.Run("update", func(t *testing.T) {
		require.ErrorIs(t, repo.Update(ctx, uuid.Nil, validTournament(t)), app.ErrUnauthenticated)
	})
	t.Run("locked update", func(t *testing.T) {
		_, err := repo.UpdateLocked(ctx, uuid.Nil, id, nil)
		require.ErrorIs(t, err, app.ErrUnauthenticated)
	})
	t.Run("delete", func(t *testing.T) { require.ErrorIs(t, repo.Delete(ctx, uuid.Nil, id), app.ErrUnauthenticated) })
	t.Run("list", func(t *testing.T) {
		_, err := repo.List(ctx, uuid.Nil, 50, 0)
		require.ErrorIs(t, err, app.ErrUnauthenticated)
	})
}

func TestTournamentRepository_OwnerIsolationIntegration(t *testing.T) {
	ctx, pool, repo := setupTournamentRepositoryTest(t)
	other := domain.User{ID: uuid.New(), Login: "other-" + uuid.New().String()[:16], PasswordHash: "test-hash", CreatedAt: time.Now().UTC()}
	require.NoError(t, NewUserRepository(pool).Create(ctx, other))
	card := validTournament(t)
	id, err := createRegisteredTournament(t, ctx, repo, card)
	require.NoError(t, err)
	_, err = repo.GetByID(ctx, other.ID, id)
	require.ErrorIs(t, err, app.ErrTournamentNotFound)
	changed := *card
	changed.Name = "Foreign Change"
	require.ErrorIs(t, repo.Update(ctx, other.ID, &changed), app.ErrTournamentNotFound)
	called := false
	_, err = repo.UpdateLocked(ctx, other.ID, id, func(current *domain.Tournament) (*domain.Tournament, error) { called = true; return current, nil })
	require.ErrorIs(t, err, app.ErrTournamentNotFound)
	assert.False(t, called)
	require.ErrorIs(t, repo.Delete(ctx, other.ID, id), app.ErrTournamentNotFound)
	otherCards, err := repo.List(ctx, other.ID, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, otherCards)
	got, err := repo.GetByID(ctx, testOwnerID, id)
	require.NoError(t, err)
	assert.Equal(t, card, got)
	ownCards, err := repo.List(ctx, testOwnerID, 50, 0)
	require.NoError(t, err)
	require.Len(t, ownCards, 1)
	assert.Equal(t, card, ownCards[0])
}
