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
	"github.com/jackc/pgx/v5/pgxpool"
)

type TournamentRepository struct {
	pool *pgxpool.Pool
}

func NewTournamentRepository(pool *pgxpool.Pool) *TournamentRepository {
	return &TournamentRepository{pool: pool}
}

func statusToString(status domain.Status) (string, error) {
	switch status {
	case domain.StatusCreated:
		return "created", nil
	case domain.StatusRunning:
		return "running", nil
	case domain.StatusPaused:
		return "paused", nil
	case domain.StatusFinished:
		return "finished", nil
	default:
		return "", errUnknownTournamentStatus
	}
}

func (r *TournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) (uuid.UUID, error) {
	if tournament == nil {
		return uuid.Nil, errNilTournament
	}
	status, err := statusToString(tournament.Status)
	if err != nil {
		return uuid.Nil, err
	}

	if tournament.ID == uuid.Nil {
		tournament.ID = uuid.New()
	}

	data, err := json.Marshal(tournament)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal tournament: %w", err)
	}

	createQuery := `
	INSERT INTO tournaments (id, name, tournament_date, status, data)
	VALUES ($1, $2, $3, $4, $5)
	`

	_, err = r.pool.Exec(ctx, createQuery, tournament.ID, tournament.Name, tournament.Date, status, data)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert tournament: %w", err)
	}

	return tournament.ID, nil
}

func (r *TournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}

	var data []byte

	sqlQuery := `
		SELECT data FROM tournaments
		WHERE id = $1
	`

	if err := r.pool.QueryRow(ctx, sqlQuery, id).Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, app.ErrTournamentNotFound
		}
		return nil, fmt.Errorf("get tournament: %w", err)
	}

	var tr domain.Tournament

	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal tournament: %w", err)
	}

	tr.ID = id

	return &tr, nil
}

func (r *TournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	if tournament == nil {
		return errNilTournament
	}
	if tournament.ID == uuid.Nil {
		return app.ErrInvalidTournamentID
	}

	status, err := statusToString(tournament.Status)
	if err != nil {
		return err
	}

	data, err := json.Marshal(tournament)
	if err != nil {
		return fmt.Errorf("marshal tournament: %w", err)
	}

	sqlQuery := `
		UPDATE tournaments
		SET name = $1,
		    tournament_date = $2,
		    status = $3,
		    data = $4,
		    updated_at = NOW()
		WHERE id = $5
	`

	result, execErr := r.pool.Exec(ctx, sqlQuery, tournament.Name, tournament.Date, status, data, tournament.ID)
	if execErr != nil {
		return fmt.Errorf("update tournament: %w", execErr)
	}
	if result.RowsAffected() == 0 {
		return app.ErrTournamentNotFound
	}

	return nil
}

func (r *TournamentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return app.ErrInvalidTournamentID
	}

	sqlQuery := `
	DELETE FROM tournaments
	WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, sqlQuery, id)
	if err != nil {
		return fmt.Errorf("delete tournament: %w", err)
	}
	if result.RowsAffected() == 0 {
		return app.ErrTournamentNotFound
	}

	return nil
}
