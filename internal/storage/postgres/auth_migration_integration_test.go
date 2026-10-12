package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMigration_ExistingCardsRollbackIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("integration test skipped: TEST_DATABASE_URL is not set")
	}
	config, err := parseTestPostgresURL(url)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.ConnectConfig(ctx, config.ConnConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = conn.Close(cleanup)
	})
	require.NoError(t, verifyTournamentTestDatabase(ctx, config.ConnConfig.Database, conn))
	schema := pgx.Identifier{"auth_migration_" + uuid.NewString()}.Sanitize()
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = conn.Exec(cleanup, "ROLLBACK")
		_, _ = conn.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE")
	})
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, migration := range []string{"000001_create_tournaments.up.sql", "000002_drop_tournament_status.up.sql"} {
		source, readErr := os.ReadFile(filepath.Join("..", "..", "..", "migrations", migration))
		require.NoError(t, readErr)
		_, err = conn.Exec(ctx, string(source))
		require.NoError(t, err)
	}
	_, err = conn.Exec(ctx, "INSERT INTO tournaments (id, name, tournament_date, data) VALUES ($1, 'legacy', NOW(), '{}'::jsonb)", uuid.New())
	require.NoError(t, err)
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000003_users_and_tournament_owners.up.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(source))
	require.Error(t, err)
	_, err = conn.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var count int
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM tournaments").Scan(&count))
	assert.Equal(t, 1, count)
	var userTableExists bool
	require.NoError(t, conn.QueryRow(ctx, "SELECT to_regclass('users') IS NOT NULL").Scan(&userTableExists))
	assert.False(t, userTableExists)
	var columnExists bool
	require.NoError(t, conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'tournaments' AND column_name = 'user_id')").Scan(&columnExists))
	assert.False(t, columnExists)
}
