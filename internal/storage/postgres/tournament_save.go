package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *TournamentRepository) SaveWithPlayers(ctx context.Context, owner, id uuid.UUID, selections []domain.TournamentPlayer, build func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	creating := id == uuid.Nil
	if creating {
		id = uuid.New()
	}
	return r.saveWithPlayers(ctx, owner, id, creating, selections, build)
}

func (r *TournamentRepository) saveWithPlayers(ctx context.Context, owner, id uuid.UUID, creating bool, selections []domain.TournamentPlayer, build func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	if owner == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	for attempt := 0; ; attempt++ {
		card, err := r.savePlayerTransaction(ctx, owner, id, creating, selections, build)
		var pgErr *pgconn.PgError
		retry := errors.Is(err, errPlayerResolutionChanged) || (errors.As(err, &pgErr) && (pgErr.Code == "40P01" || pgErr.Code == "40001"))
		if !retry || attempt >= 2 || ctx.Err() != nil {
			return card, err
		}
	}
}

func (r *TournamentRepository) savePlayerTransaction(ctx context.Context, owner, id uuid.UUID, creating bool, selections []domain.TournamentPlayer, build func([]domain.TournamentPlayer, *domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tournament save: %w", err)
	}
	defer rollbackTournamentTx(ctx, tx)
	var original *domain.Tournament
	if !creating {
		original, err = scanTournament(tx.QueryRow(ctx, `SELECT data FROM tournaments WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, owner), id)
		if err != nil {
			return nil, fmt.Errorf("lock tournament: %w", err)
		}
	}
	resolved, err := resolveTournamentPlayers(ctx, tx, owner, selections, original)
	if err != nil {
		return nil, err
	}
	card, err := build(resolved, original)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, errNilTournament
	}
	if card.ID != uuid.Nil && card.ID != id {
		return nil, fmt.Errorf("changed tournament id does not match locked row")
	}
	card.ID = id
	if original != nil {
		card.BuyInAmount = original.BuyInAmount
	}
	if len(card.Players) != len(resolved) {
		return nil, errors.Join(app.ErrValidation, domain.ErrInvalidPlayerID)
	}
	for i := range card.Players {
		if card.Players[i].ID != resolved[i].ID {
			return nil, errors.Join(app.ErrValidation, domain.ErrInvalidPlayerID)
		}
		card.Players[i].Name, card.Players[i].IsDeleted = resolved[i].Name, resolved[i].IsDeleted
	}
	if creating {
		data, err := json.Marshal(card)
		if err != nil {
			return nil, fmt.Errorf("marshal tournament: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tournaments (id, name, tournament_date, data, user_id) VALUES ($1, $2, $3, $4, $5)`, id, card.Name, card.Date, data, owner); err != nil {
			return nil, fmt.Errorf("insert tournament: %w", err)
		}
	} else if err := updateTournament(ctx, tx, owner, card); err != nil {
		return nil, err
	}
	if err := syncTournamentPlayers(ctx, tx, owner, card); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tournament save: %w", err)
	}
	return card, nil
}

func syncTournamentPlayers(ctx context.Context, tx pgx.Tx, owner uuid.UUID, card *domain.Tournament) error {
	if _, err := tx.Exec(ctx, `DELETE FROM tournament_players WHERE tournament_id = $1 AND user_id = $2`, card.ID, owner); err != nil {
		return fmt.Errorf("clear tournament players: %w", err)
	}
	ids := make([]uuid.UUID, len(card.Players))
	for i, player := range card.Players {
		ids[i] = player.ID
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tournament_players (tournament_id, player_id, user_id) SELECT $1, player_id, $3 FROM unnest($2::uuid[]) AS player_id`, card.ID, ids, owner); err != nil {
		return fmt.Errorf("link tournament players: %w", err)
	}
	return nil
}
