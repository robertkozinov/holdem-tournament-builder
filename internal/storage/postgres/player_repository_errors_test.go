package postgres

import (
	"context"
	"errors"
	"testing"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanPlayer_Errors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cause, want error
	}{
		{"missing", pgx.ErrNoRows, app.ErrPlayerNotFound},
		{"name conflict", &pgconn.PgError{Code: "23505", ConstraintName: "players_owner_name_active_key"}, app.ErrPlayerNameTaken},
		{"other conflict", &pgconn.PgError{Code: "23505", ConstraintName: "players_pkey"}, nil},
		{"storage error", errors.New("storage failed"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			player, err := scanPlayer(tournamentRowFunc(func(...any) error { return tc.cause }))
			require.Error(t, err)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.ErrorIs(t, err, tc.cause)
			}
			assert.Equal(t, domain.Player{}, player)
		})
	}
}

type playerErrorDB struct{ cause error }

func (d *playerErrorDB) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, d.cause }
func (d *playerErrorDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return tournamentRowFunc(func(...any) error { return d.cause })
}

func TestPlayerRepository_PreservesStorageErrors(t *testing.T) {
	cause := errors.New("connection failed")
	r := &PlayerRepository{pool: &playerErrorDB{cause: cause}}
	ctx, owner, id := context.Background(), uuid.New(), uuid.New()
	_, err := r.Create(ctx, owner, registryPlayer("Влад"))
	require.ErrorIs(t, err, cause)
	_, err = r.List(ctx, owner, "", 10, 0)
	require.ErrorIs(t, err, cause)
	_, err = r.Rename(ctx, owner, id, "Влад", "влад", registryPlayer("Влад").CreatedAt)
	require.ErrorIs(t, err, cause)
	require.ErrorIs(t, r.Delete(ctx, owner, id, registryPlayer("Влад").CreatedAt), cause)
}
