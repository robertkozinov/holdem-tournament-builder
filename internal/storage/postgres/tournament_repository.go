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

func (r *TournamentRepository) Create(ctx context.Context, ownerID uuid.UUID, tournament *domain.Tournament) (uuid.UUID, error) {
	if ownerID == uuid.Nil {
		return uuid.Nil, app.ErrUnauthenticated
	}
	if tournament == nil {
		return uuid.Nil, errNilTournament
	}
	id := tournament.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	card, err := r.saveWithPlayers(ctx, ownerID, id, true, tournament.Players, func(resolved []domain.TournamentPlayer, _ *domain.Tournament) (*domain.Tournament, error) {
		candidate := *tournament
		candidate.ID, candidate.Players = id, resolved
		card, err := domain.NewTournament(candidate)
		if err != nil {
			return nil, errors.Join(app.ErrValidation, err)
		}
		return card, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	*tournament = *card
	return card.ID, nil
}

func (r *TournamentRepository) GetByID(ctx context.Context, ownerID, id uuid.UUID) (*domain.Tournament, error) {
	if ownerID == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}

	sqlQuery := `
		SELECT data FROM tournaments
		WHERE id = $1 AND user_id = $2
	`

	card, err := scanTournament(r.pool.QueryRow(ctx, sqlQuery, id, ownerID), id)
	if err != nil {
		return nil, err
	}
	cards := []*domain.Tournament{card}
	profiles, err := loadTournamentProfiles(ctx, r.pool, ownerID, tournamentPlayerIDs(cards), false)
	if err != nil {
		return nil, err
	}
	if err := hydrateTournamentPlayers(cards, profiles); err != nil {
		return nil, err
	}
	return card, nil
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

func (r *TournamentRepository) List(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.Tournament, error) {
	if ownerID == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, data FROM tournaments
		WHERE user_id = $1
		ORDER BY tournament_date DESC, id DESC
		LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)
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

	rows.Close()
	profiles, err := loadTournamentProfiles(ctx, r.pool, ownerID, tournamentPlayerIDs(tournaments), false)
	if err != nil {
		return nil, err
	}
	if err := hydrateTournamentPlayers(tournaments, profiles); err != nil {
		return nil, err
	}
	return tournaments, nil
}

func (r *TournamentRepository) Update(ctx context.Context, ownerID uuid.UUID, tournament *domain.Tournament) error {
	if ownerID == uuid.Nil {
		return app.ErrUnauthenticated
	}
	if tournament == nil {
		return errNilTournament
	}
	if tournament.ID == uuid.Nil {
		return app.ErrInvalidTournamentID
	}
	card, err := r.saveWithPlayers(ctx, ownerID, tournament.ID, false, tournament.Players, func(resolved []domain.TournamentPlayer, original *domain.Tournament) (*domain.Tournament, error) {
		candidate := *tournament
		candidate.Players, candidate.BuyInAmount = resolved, original.BuyInAmount
		card, err := domain.NewTournament(candidate)
		if err != nil {
			return nil, errors.Join(app.ErrValidation, err)
		}
		return card, nil
	})
	if err != nil {
		return err
	}
	*tournament = *card
	return nil
}

func updateTournament(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, ownerID uuid.UUID, tournament *domain.Tournament) error {
	if ownerID == uuid.Nil {
		return app.ErrUnauthenticated
	}
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
		WHERE id = $4 AND user_id = $5
	`

	result, execErr := db.Exec(ctx, sqlQuery, tournament.Name, tournament.Date, data, tournament.ID, ownerID)
	if execErr != nil {
		return fmt.Errorf("update tournament: %w", execErr)
	}
	if result.RowsAffected() == 0 {
		return app.ErrTournamentNotFound
	}

	return nil
}

func (r *TournamentRepository) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	if ownerID == uuid.Nil {
		return app.ErrUnauthenticated
	}
	if id == uuid.Nil {
		return app.ErrInvalidTournamentID
	}

	sqlQuery := `
	DELETE FROM tournaments
	WHERE id = $1 AND user_id = $2
	`

	result, err := r.pool.Exec(ctx, sqlQuery, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete tournament: %w", err)
	}
	if result.RowsAffected() == 0 {
		return app.ErrTournamentNotFound
	}

	return nil
}
