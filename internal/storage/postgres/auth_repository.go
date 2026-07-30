package postgres

import (
	"context"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

func (r *AuthRepository) CreateUserWithSession(ctx context.Context, user domain.User, session domain.Session) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	insertUserQuery := `
		INSERT INTO users (
			id,
			username,
			password_hash,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = tx.Exec(ctx, insertUserQuery, user.ID, user.Username, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_username_key" {
			return app.ErrUsernameAlreadyExists
		}

		return fmt.Errorf("insert user: %w", err)
	}

	insertSessionQuery := `
		INSERT INTO sessions (
			token_hash,
			user_id,
			expires_at,
			created_at
		)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(ctx, insertSessionQuery, session.TokenHash, session.UserID, session.ExpiresAt, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
