package postgres

import (
	"context"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTournamentRepositoryTest(t *testing.T) (context.Context, *pgxpool.Pool, *TournamentRepository) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE tournaments")
	require.NoError(t, err)

	repo := NewTournamentRepository(pool)

	return ctx, pool, repo
}

func validTournament(t *testing.T) *domain.Tournament {
	t.Helper()

	tr, err := domain.NewTournament(
		"Friday Game",
		time.Date(2026, 7, 8, 20, 0, 0, 0, time.UTC),
		[]string{"A", "B"},
		1000,
		[]domain.ChipDenomination{{Value: 25, Count: 100}},
		domain.RebuyRules{Allowed: true, MaxLevel: 3},
		2*time.Hour,
		domain.StackPlan{},
		nil,
		nil,
	)
	require.NoError(t, err)

	return tr
}

func TestTournamentRepository_Create(t *testing.T) {
	t.Run("creates tournament", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)

		id, err := repo.Create(ctx, tr)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id)
		assert.Equal(t, id, tr.ID)

		var name string
		var storedID uuid.UUID

		err = pool.QueryRow(ctx,
			"SELECT id, name FROM tournaments WHERE id = $1",
			id,
		).Scan(&storedID, &name)
		require.NoError(t, err)

		assert.Equal(t, id, storedID)
		assert.Equal(t, "Friday Game", name)
	})
	t.Run("returns errors when status is unknown", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)

		tr.Status = domain.Status(999)

		id, err := repo.Create(ctx, tr)
		require.ErrorIs(t, err, errUnknownTournamentStatus)
		assert.Equal(t, uuid.Nil, id)
	})
	t.Run("returns error when id already exists", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		id := uuid.New()

		first := validTournament(t)
		first.ID = id

		second := validTournament(t)
		second.ID = id

		createdID, err := repo.Create(ctx, first)
		require.NoError(t, err)
		assert.Equal(t, id, createdID)

		newID, err := repo.Create(ctx, second)
		require.Error(t, err)
		assert.Equal(t, uuid.Nil, newID)
	})
}

func TestTournamentRepository_GetByID(t *testing.T) {
	t.Run("gets tournament", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)
		tr := validTournament(t)

		createdID, err := repo.Create(ctx, tr)
		require.NoError(t, err)

		gotTr, err := repo.GetByID(ctx, createdID)
		require.NoError(t, err)

		assert.Equal(t, createdID, gotTr.ID)
		assert.Equal(t, tr.Name, gotTr.Name)
		assert.Equal(t, tr.Players, gotTr.Players)
		assert.Equal(t, tr.BuyInAmount, gotTr.BuyInAmount)
	})
	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		gotTr, err := repo.GetByID(ctx, uuid.New())

		require.ErrorIs(t, err, app.ErrTournamentNotFound)
		assert.Nil(t, gotTr)
	})
	t.Run("returns error when id is empty", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		gotTr, err := repo.GetByID(ctx, uuid.Nil)

		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
		assert.Nil(t, gotTr)
	})
}

func TestTournamentRepository_Update(t *testing.T) {
	t.Run("updates tournament", func(t *testing.T) {
		ctx, pool, repo := setupTournamentRepositoryTest(t)

		tr := validTournament(t)

		createdID, err := repo.Create(ctx, tr)
		require.NoError(t, err)

		tr.Name = "Saturday Game"
		tr.Status = domain.StatusRunning

		updateErr := repo.Update(ctx, tr)
		require.NoError(t, updateErr)

		gotTr, err := repo.GetByID(ctx, createdID)
		require.NoError(t, err)

		assert.Equal(t, createdID, gotTr.ID)
		assert.Equal(t, "Saturday Game", gotTr.Name)
		assert.Equal(t, domain.StatusRunning, gotTr.Status)

		var storedName string
		var storedStatus string
		var storedDate time.Time

		err = pool.QueryRow(ctx, `
			SELECT name, status, tournament_date
			FROM tournaments
			WHERE id = $1
		`, createdID).Scan(&storedName, &storedStatus, &storedDate)
		require.NoError(t, err)

		assert.Equal(t, "Saturday Game", storedName)
		assert.Equal(t, "running", storedStatus)
		assert.True(t, tr.Date.Equal(storedDate), "expected %v, got %v", tr.Date, storedDate)
	})

	t.Run("returns error when tournament is nil", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		err := repo.Update(ctx, nil)
		require.ErrorIs(t, err, errNilTournament)
	})

	t.Run("returns error when id is empty", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		tr := validTournament(t)
		tr.ID = uuid.Nil

		updateErr := repo.Update(ctx, tr)
		require.ErrorIs(t, updateErr, app.ErrInvalidTournamentID)
	})

	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		tr := validTournament(t)
		tr.ID = uuid.New()

		updateErr := repo.Update(ctx, tr)
		require.ErrorIs(t, updateErr, app.ErrTournamentNotFound)
	})

	t.Run("returns error when status is unknown", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		tr := validTournament(t)
		tr.ID = uuid.New()
		tr.Status = domain.Status(999)

		updateErr := repo.Update(ctx, tr)
		require.ErrorIs(t, updateErr, errUnknownTournamentStatus)
	})
}

func TestTournamentRepository_Delete(t *testing.T) {
	t.Run("deletes tournament", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		tr := validTournament(t)

		createdID, err := repo.Create(ctx, tr)
		require.NoError(t, err)

		deleteErr := repo.Delete(ctx, createdID)
		require.NoError(t, deleteErr)

		gotTr, getErr := repo.GetByID(ctx, createdID)
		require.ErrorIs(t, getErr, app.ErrTournamentNotFound)
		assert.Nil(t, gotTr)
	})

	t.Run("returns error when id is empty", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		err := repo.Delete(ctx, uuid.Nil)
		require.ErrorIs(t, err, app.ErrInvalidTournamentID)
	})

	t.Run("returns not found when tournament does not exist", func(t *testing.T) {
		ctx, _, repo := setupTournamentRepositoryTest(t)

		deleteErr := repo.Delete(ctx, uuid.New())
		require.ErrorIs(t, deleteErr, app.ErrTournamentNotFound)
	})
}
