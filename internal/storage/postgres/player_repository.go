package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type playerDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PlayerRepository struct{ pool playerDB }

func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository { return &PlayerRepository{pool: pool} }

func (r *PlayerRepository) Create(ctx context.Context, owner uuid.UUID, player domain.Player) (domain.Player, error) {
	if owner == uuid.Nil {
		return domain.Player{}, app.ErrUnauthenticated
	}
	return scanPlayer(r.pool.QueryRow(ctx, `
		INSERT INTO players (id, user_id, name, normalized_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, normalized_name, created_at, updated_at, deleted_at
	`, player.ID, owner, player.Name, player.NormalizedName, player.CreatedAt, player.UpdatedAt))
}

func (r *PlayerRepository) List(ctx context.Context, owner uuid.UUID, query string, limit, offset int) ([]domain.Player, error) {
	if owner == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, normalized_name, created_at, updated_at, deleted_at FROM players
		WHERE user_id = $1 AND deleted_at IS NULL AND strpos(normalized_name, $2) > 0
		ORDER BY normalized_name, id LIMIT $3 OFFSET $4
	`, owner, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query players: %w", err)
	}
	defer rows.Close()
	players := make([]domain.Player, 0)
	for rows.Next() {
		player, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate players: %w", err)
	}
	return players, nil
}

func (r *PlayerRepository) Rename(ctx context.Context, owner, id uuid.UUID, name, normalized string, now time.Time) (domain.Player, error) {
	if owner == uuid.Nil {
		return domain.Player{}, app.ErrUnauthenticated
	}
	return scanPlayer(r.pool.QueryRow(ctx, `
		UPDATE players SET name = $1, normalized_name = $2, updated_at = $3
		WHERE id = $4 AND user_id = $5 AND deleted_at IS NULL
		RETURNING id, name, normalized_name, created_at, updated_at, deleted_at
	`, name, normalized, now, id, owner))
}

func (r *PlayerRepository) Delete(ctx context.Context, owner, id uuid.UUID, now time.Time) error {
	if owner == uuid.Nil {
		return app.ErrUnauthenticated
	}
	var deletedID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		UPDATE players SET deleted_at = COALESCE(deleted_at, $1),
		updated_at = CASE WHEN deleted_at IS NULL THEN $1 ELSE updated_at END
		WHERE id = $2 AND user_id = $3 RETURNING id
	`, now, id, owner).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrPlayerNotFound
	}
	if err != nil {
		return fmt.Errorf("delete player: %w", err)
	}
	return nil
}

func scanPlayer(row pgx.Row) (domain.Player, error) {
	var player domain.Player
	if err := row.Scan(&player.ID, &player.Name, &player.NormalizedName, &player.CreatedAt, &player.UpdatedAt, &player.DeletedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Player{}, app.ErrPlayerNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "players_owner_name_active_key" {
			return domain.Player{}, fmt.Errorf("%w: %w", app.ErrPlayerNameTaken, err)
		}
		return domain.Player{}, fmt.Errorf("scan player: %w", err)
	}
	player.CreatedAt, player.UpdatedAt = player.CreatedAt.UTC(), player.UpdatedAt.UTC()
	if player.DeletedAt != nil {
		utc := player.DeletedAt.UTC()
		player.DeletedAt = &utc
	}
	return player, nil
}
