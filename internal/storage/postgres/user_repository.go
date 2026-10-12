package postgres

import (
	"context"
	"errors"
	"fmt"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type UserRepository struct{ pool userDB }

func NewUserRepository(pool *pgxpool.Pool) *UserRepository { return &UserRepository{pool: pool} }

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	if user.ID == uuid.Nil || user.Login == "" || user.PasswordHash == "" || user.CreatedAt.IsZero() {
		return app.ErrValidation
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, created_at) VALUES ($1, $2, $3, $4)`, user.ID, user.Login, user.PasswordHash, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_login_key" {
			return fmt.Errorf("%w: %w", app.ErrLoginTaken, err)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByLogin(ctx context.Context, login string) (domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT id, login, password_hash, created_at FROM users WHERE login = $1`, login))
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if id == uuid.Nil {
		return domain.User{}, app.ErrUserNotFound
	}
	return scanUser(r.pool.QueryRow(ctx, `SELECT id, login, password_hash, created_at FROM users WHERE id = $1`, id))
}

func scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User
	if err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, app.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	user.CreatedAt = user.CreatedAt.UTC()
	return user, nil
}
