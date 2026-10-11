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
	"github.com/jackc/pgx/v5/pgxpool"
)

type tournamentDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type TournamentRepository struct {
	pool tournamentDB
}

func NewTournamentRepository(pool *pgxpool.Pool) *TournamentRepository {
	return &TournamentRepository{pool: pool}
}

func (r *TournamentRepository) Create(ctx context.Context, tournament *domain.Tournament) (uuid.UUID, error) {
	if tournament == nil {
		return uuid.Nil, errNilTournament
	}
	if tournament.ID == uuid.Nil {
		tournament.ID = uuid.New()
	}

	data, err := json.Marshal(tournament)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal tournament: %w", err)
	}

	createQuery := `
	INSERT INTO tournaments (id, name, tournament_date, data)
	VALUES ($1, $2, $3, $4)
	`

	_, err = r.pool.Exec(ctx, createQuery, tournament.ID, tournament.Name, tournament.Date, data)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert tournament: %w", err)
	}

	return tournament.ID, nil
}

func (r *TournamentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}

	sqlQuery := `
		SELECT data FROM tournaments
		WHERE id = $1
	`

	return scanTournament(r.pool.QueryRow(ctx, sqlQuery, id), id)
}

func scanTournament(row pgx.Row, id uuid.UUID) (*domain.Tournament, error) {
	var data []byte
	if err := row.Scan(&data); err != nil {
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

func (r *TournamentRepository) List(ctx context.Context, limit, offset int) ([]*domain.Tournament, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, data FROM tournaments
		ORDER BY tournament_date DESC, id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*domain.Tournament, 0)
	for rows.Next() {
		var id uuid.UUID
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("scan tournament: %w", err)
		}

		var tournament domain.Tournament
		if err := json.Unmarshal(data, &tournament); err != nil {
			return nil, fmt.Errorf("unmarshal tournament: %w", err)
		}
		tournament.ID = id
		tournaments = append(tournaments, &tournament)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tournaments: %w", err)
	}

	return tournaments, nil
}

func (r *TournamentRepository) Update(ctx context.Context, tournament *domain.Tournament) error {
	return updateTournament(ctx, r.pool, tournament)
}

func updateTournament(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, tournament *domain.Tournament) error {
	if tournament == nil {
		return errNilTournament
	}
	if tournament.ID == uuid.Nil {
		return app.ErrInvalidTournamentID
	}

	data, err := json.Marshal(tournament)
	if err != nil {
		return fmt.Errorf("marshal tournament: %w", err)
	}

	sqlQuery := `
		UPDATE tournaments
		SET name = $1,
		    tournament_date = $2,
		    data = $3,
		    updated_at = NOW()
		WHERE id = $4
	`

	result, execErr := db.Exec(ctx, sqlQuery, tournament.Name, tournament.Date, data, tournament.ID)
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
