package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerRegistryMigration_Integration(t *testing.T) {
	endpoint := os.Getenv("TEST_DATABASE_URL")
	if endpoint == "" {
		t.Skip("integration test skipped: TEST_DATABASE_URL is not set")
	}
	cfg, err := parseTestPostgresURL(endpoint)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(cfg.ConnConfig.Database, "_test"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	t.Cleanup(conn.Release)
	require.NoError(t, verifyTournamentTestDatabase(ctx, cfg.ConnConfig.Database, conn))
	schema := pgx.Identifier{"registry_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := conn.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE")
		assert.NoError(t, err)
	})
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	apply := func(file string) error {
		source, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", file))
		if err != nil {
			return err
		}
		_, err = conn.Exec(ctx, string(source))
		return err
	}
	for _, name := range []string{"000001_create_tournaments.up.sql", "000002_drop_tournament_status.up.sql", "000003_users_and_tournament_owners.up.sql", "000004_player_registry.up.sql"} {
		require.NoError(t, apply(name))
	}
	owner, foreign := uuid.New(), uuid.New()
	_, err = conn.Exec(ctx, `INSERT INTO users (id,login,password_hash,created_at) VALUES ($1,'first','test-hash',NOW()),($2,'second','test-hash',NOW())`, owner, foreign)
	require.NoError(t, err)
	card, player := uuid.New(), uuid.New()
	_, err = conn.Exec(ctx, `INSERT INTO tournaments (id,name,tournament_date,data,user_id) VALUES ($1,'test',NOW(),'{}',$2)`, card, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO players (id,user_id,name,normalized_name,created_at,updated_at) VALUES ($1,$2,'Влад','влад',NOW(),NOW())`, player, foreign)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO tournament_players (tournament_id,player_id,user_id) VALUES ($1,$2,$3)`, card, player, owner)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23503", pgErr.Code)
	_, err = conn.Exec(ctx, `INSERT INTO tournament_players (tournament_id,player_id,user_id) VALUES ($1,$2,$3)`, card, player, foreign)
	require.Error(t, err)
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23503", pgErr.Code)
	require.NoError(t, apply("000004_player_registry.down.sql"))
	var playersTable *string
	require.NoError(t, conn.QueryRow(ctx, "SELECT to_regclass('players')::text").Scan(&playersTable))
	assert.Nil(t, playersTable)
}
