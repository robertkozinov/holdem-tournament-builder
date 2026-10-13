package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlayerRepositoryTest(t *testing.T) (context.Context, *pgxpool.Pool, *PlayerRepository, uuid.UUID) {
	t.Helper()
	endpoint := os.Getenv("TEST_DATABASE_URL")
	if endpoint == "" {
		t.Skip("integration test skipped: TEST_DATABASE_URL is not set")
	}
	config, err := parseTestPostgresURL(endpoint)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	err = verifyTournamentTestDatabase(ctx, config.ConnConfig.Database, conn)
	conn.Release()
	require.NoError(t, err)
	owner := uuid.New()
	require.NoError(t, NewUserRepository(pool).Create(ctx, domain.User{ID: owner, Login: "player_" + owner.String()[:8], PasswordHash: "test-hash", CreatedAt: time.Now().UTC()}))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, "DELETE FROM tournaments WHERE user_id = $1", owner)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, "DELETE FROM players WHERE user_id = $1", owner)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", owner)
		assert.NoError(t, err)
	})
	return ctx, pool, NewPlayerRepository(pool), owner
}

func registryPlayer(name string) domain.Player {
	display, normalized, _ := domain.NormalizePlayerName(name)
	now := time.Now().UTC()
	return domain.Player{ID: uuid.New(), Name: display, NormalizedName: normalized, CreatedAt: now, UpdatedAt: now}
}

func TestPlayerRepository_CRUDIntegration(t *testing.T) {
	ctx, _, repo, owner := setupPlayerRepositoryTest(t)
	p, err := repo.Create(ctx, owner, registryPlayer("  Влад  "))
	require.NoError(t, err)
	assert.Equal(t, "Влад", p.Name)
	_, err = repo.Create(ctx, owner, registryPlayer("ВЛАД"))
	require.ErrorIs(t, err, app.ErrPlayerNameTaken)
	items, err := repo.List(ctx, owner, "вл", 50, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, p.ID, items[0].ID)
	renamed, err := repo.Rename(ctx, owner, p.ID, "Влад Фед", "влад фед", time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, p.ID, renamed.ID)
	assert.Equal(t, p.CreatedAt, renamed.CreatedAt)
	items, err = repo.List(ctx, owner, "фед", 50, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Влад Фед", items[0].Name)
	other, err := repo.Create(ctx, owner, registryPlayer("Влад Поп"))
	require.NoError(t, err)
	_, err = repo.Rename(ctx, owner, other.ID, "Влад Фед", "влад фед", time.Now().UTC())
	require.ErrorIs(t, err, app.ErrPlayerNameTaken)
	require.NoError(t, repo.Delete(ctx, owner, p.ID, time.Now().UTC()))
	items, err = repo.List(ctx, owner, "фед", 50, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.NotNil(t, items)
	_, err = repo.Rename(ctx, owner, p.ID, "After delete", "after delete", time.Now().UTC())
	require.ErrorIs(t, err, app.ErrPlayerNotFound)
	require.NoError(t, repo.Delete(ctx, owner, p.ID, time.Now().UTC()))
	recreated, err := repo.Create(ctx, owner, registryPlayer("Влад Фед"))
	require.NoError(t, err)
	assert.NotEqual(t, p.ID, recreated.ID)
}

func TestPlayerRepository_OwnershipIntegration(t *testing.T) {
	ctx, _, repo, owner := setupPlayerRepositoryTest(t)
	_, _, _, foreign := setupPlayerRepositoryTest(t)
	p, err := repo.Create(ctx, owner, registryPlayer("Влад"))
	require.NoError(t, err)
	_, err = repo.Create(ctx, foreign, registryPlayer("влад"))
	require.NoError(t, err)
	_, err = repo.Rename(ctx, foreign, p.ID, "Чужой", "чужой", time.Now().UTC())
	require.ErrorIs(t, err, app.ErrPlayerNotFound)
	require.ErrorIs(t, repo.Delete(ctx, foreign, p.ID, time.Now().UTC()), app.ErrPlayerNotFound)
	items, err := repo.List(ctx, foreign, "", 50, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.NotEqual(t, p.ID, items[0].ID)
}

func TestPlayerRepository_SearchLiteralIntegration(t *testing.T) {
	ctx, _, repo, owner := setupPlayerRepositoryTest(t)
	for _, name := range []string{"Player_7", "Player%8", "Player9", "Влад Поп", "Влад Фед"} {
		_, err := repo.Create(ctx, owner, registryPlayer(name))
		require.NoError(t, err)
	}
	for _, query := range []string{"_", "%", "фед"} {
		items, err := repo.List(ctx, owner, query, 10, 0)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Contains(t, items[0].NormalizedName, query)
	}
	first, err := repo.List(ctx, owner, "влад", 1, 0)
	require.NoError(t, err)
	second, err := repo.List(ctx, owner, "влад", 1, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.NotEqual(t, first[0].ID, second[0].ID)
}

func TestPlayerRepository_ConcurrentUniqueIntegration(t *testing.T) {
	ctx, _, repo, owner := setupPlayerRepositoryTest(t)
	const count = 8
	results := make(chan error, count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Create(ctx, owner, registryPlayer("Влад"))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, app.ErrPlayerNameTaken)
		}
	}
	assert.Equal(t, 1, success)
}

func TestPlayerRepository_RejectsMissingOwner(t *testing.T) {
	r := NewPlayerRepository(nil)
	ctx := context.Background()
	_, err := r.Create(ctx, uuid.Nil, registryPlayer("Влад"))
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	_, err = r.List(ctx, uuid.Nil, "", 50, 0)
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	_, err = r.Rename(ctx, uuid.Nil, uuid.New(), "Влад", "влад", time.Now())
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	require.ErrorIs(t, r.Delete(ctx, uuid.Nil, uuid.New(), time.Now()), app.ErrUnauthenticated)
}
