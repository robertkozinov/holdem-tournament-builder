package postgres

import (
	"context"
	"fmt"

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
	defer rollbackTournamentTx(ctx, tx)

	original, err := scanTournament(tx.QueryRow(ctx, `
		SELECT data FROM tournaments
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, id, ownerID), id)
	if err != nil {
		return nil, fmt.Errorf("lock tournament: %w", err)
	}
	ids := tournamentPlayerIDs([]*domain.Tournament{original})
	profiles, err := loadTournamentProfiles(ctx, tx, ownerID, ids, true)
	if err != nil {
		return nil, err
	}
	if err := hydrateTournamentPlayers([]*domain.Tournament{original}, profiles); err != nil {
		return nil, err
	}
	buyIn := original.BuyInAmount
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
	updatedIDs := tournamentPlayerIDs([]*domain.Tournament{updated})
	if len(updatedIDs) != len(ids) {
		return nil, fmt.Errorf("locked update cannot change tournament players")
	}
	for i := range ids {
		if ids[i] != updatedIDs[i] {
			return nil, fmt.Errorf("locked update cannot change tournament players")
		}
	}
	updated.BuyInAmount = buyIn
	if err := hydrateTournamentPlayers([]*domain.Tournament{updated}, profiles); err != nil {
		return nil, err
	}
	if err := updateTournament(ctx, tx, ownerID, updated); err != nil {
		return nil, err
	}
	if err := syncTournamentPlayers(ctx, tx, ownerID, updated); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tournament update: %w", err)
	}
	return updated, nil
}
