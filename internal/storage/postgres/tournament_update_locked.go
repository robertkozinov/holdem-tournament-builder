package postgres

import (
	"context"
	"fmt"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UpdateLocked applies change under a row lock and returns only after commit.
// The callback must not access the repository or perform external operations.
func (r *TournamentRepository) UpdateLocked(ctx context.Context, ownerID, id uuid.UUID, change func(*domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	if ownerID == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tournament update: %w", err)
	}
	defer func() {
		// A cancelled request must not prevent releasing the transaction and its lock.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	original, err := scanTournament(tx.QueryRow(ctx, `
		SELECT data FROM tournaments
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, id, ownerID), id)
	if err != nil {
		return nil, fmt.Errorf("lock tournament: %w", err)
	}
	updated, err := change(original)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errNilTournament
	}
	if updated.ID != id {
		return nil, fmt.Errorf("changed tournament id does not match locked row")
	}
	if err := updateTournament(ctx, tx, ownerID, updated); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tournament update: %w", err)
	}
	return updated, nil
}
