package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func createRegisteredTournament(t *testing.T, ctx context.Context, repo *TournamentRepository, card *domain.Tournament) (uuid.UUID, error) {
	t.Helper()
	for i := range card.Players {
		oldID := card.Players[i].ID
		display, key, err := domain.NormalizePlayerName(card.Players[i].Name)
		require.NoError(t, err)
		var id uuid.UUID
		err = repo.pool.QueryRow(ctx, `INSERT INTO players (id,user_id,name,normalized_name,created_at,updated_at)
			VALUES ($1,$2,$3,$4,NOW(),NOW()) ON CONFLICT (user_id,normalized_name) WHERE deleted_at IS NULL
			DO UPDATE SET normalized_name = EXCLUDED.normalized_name RETURNING id`, oldID, testOwnerID, display, key).Scan(&id)
		require.NoError(t, err)
		card.Players[i].ID, card.Players[i].Name = id, display
		for j := range card.Results {
			if card.Results[j].PlayerID == oldID {
				card.Results[j].PlayerID = id
			}
		}
	}
	return repo.Create(ctx, testOwnerID, card)
}

func stubRegistryPlayers(cards ...*domain.Tournament) []registryTestPlayer {
	var profiles []registryTestPlayer
	for _, card := range cards {
		for _, player := range card.Players {
			_, key, _ := domain.NormalizePlayerName(player.Name)
			profiles = append(profiles, registryTestPlayer{player.ID, player.Name, key, player.IsDeleted})
		}
	}
	return profiles
}

func attachTournamentRegistry(t *testing.T, db *stubTournamentDB, cards ...*domain.Tournament) {
	t.Helper()
	profiles := stubRegistryPlayers(cards...)
	query := db.query
	db.query = func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
		if strings.Contains(sql, "FROM players") {
			return registryRows(profiles...), nil
		}
		return query(ctx, sql, args...)
	}
	row := db.queryRow
	data, err := json.Marshal(cards[0])
	require.NoError(t, err)
	db.queryRow = func(ctx context.Context, sql string, args ...any) pgx.Row {
		if row == nil && strings.Contains(sql, "SELECT data FROM tournaments") {
			return tournamentRowFunc(func(dest ...any) error { *(dest[0].(*[]byte)) = data; return nil })
		}
		return row(ctx, sql, args...)
	}
	exec := db.exec
	db.exec = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "tournament_players") {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		}
		return exec(ctx, sql, args...)
	}
}

func attachTournamentTxRegistry(tx *stubTournamentTx, card *domain.Tournament) {
	profiles := stubRegistryPlayers(card)
	tx.query = func(context.Context, string, ...any) (pgx.Rows, error) { return registryRows(profiles...), nil }
	exec := tx.exec
	tx.exec = func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		if strings.Contains(sql, "tournament_players") {
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		}
		return exec(ctx, sql, args...)
	}
}
