package postgres

import (
	"context"
	"crypto/sha256"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthRepositoryTest(t *testing.T) (context.Context, *pgxpool.Pool, *AuthRepository) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, "TRUNCATE TABLE users CASCADE")
	require.NoError(t, err)

	repo := NewAuthRepository(pool)

	return ctx, pool, repo
}

func TestAuthRepository_CreateUserWithSession(t *testing.T) {
	t.Run("creates user and session", func(t *testing.T) {
		ctx, pool, repo := setupAuthRepositoryTest(t)

		now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
		userID := uuid.New()
		tokenHash := sha256.Sum256([]byte("test-session-token"))

		user := domain.User{
			ID:           userID,
			Username:     "dealer_one",
			PasswordHash: "password-hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		session := domain.Session{
			TokenHash: tokenHash[:],
			UserID:    userID,
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		err := repo.CreateUserWithSession(ctx, user, session)
		require.NoError(t, err)

		var (
			storedUsername     string
			storedPasswordHash string
			userCreatedAt      time.Time
			userUpdatedAt      time.Time
		)

		err = pool.QueryRow(ctx, `
			SELECT username, password_hash, created_at, updated_at
			FROM users
			WHERE id = $1
		`, userID).Scan(
			&storedUsername,
			&storedPasswordHash,
			&userCreatedAt,
			&userUpdatedAt,
		)
		require.NoError(t, err)

		assert.Equal(t, user.Username, storedUsername)
		assert.Equal(t, user.PasswordHash, storedPasswordHash)
		assert.True(t, user.CreatedAt.Equal(userCreatedAt))
		assert.True(t, user.UpdatedAt.Equal(userUpdatedAt))

		var (
			storedTokenHash  []byte
			storedUserID     uuid.UUID
			sessionCreatedAt time.Time
			sessionExpiresAt time.Time
		)

		err = pool.QueryRow(ctx, `
			SELECT token_hash, user_id, created_at, expires_at
			FROM sessions
			WHERE token_hash = $1
		`, session.TokenHash).Scan(
			&storedTokenHash,
			&storedUserID,
			&sessionCreatedAt,
			&sessionExpiresAt,
		)
		require.NoError(t, err)

		assert.Equal(t, session.TokenHash, storedTokenHash)
		assert.Equal(t, session.UserID, storedUserID)
		assert.True(t, session.CreatedAt.Equal(sessionCreatedAt))
		assert.True(t, session.ExpiresAt.Equal(sessionExpiresAt))
	})

	t.Run("rolls back user when session creation fails", func(t *testing.T) {
		ctx, pool, repo := setupAuthRepositoryTest(t)

		now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
		tokenHash := sha256.Sum256([]byte("shared-session-token"))

		firstUser := domain.User{
			ID:           uuid.New(),
			Username:     "first_dealer",
			PasswordHash: "first-password-hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		firstSession := domain.Session{
			TokenHash: tokenHash[:],
			UserID:    firstUser.ID,
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		err := repo.CreateUserWithSession(ctx, firstUser, firstSession)
		require.NoError(t, err)

		secondUser := domain.User{
			ID:           uuid.New(),
			Username:     "second_dealer",
			PasswordHash: "second-password-hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		secondSession := domain.Session{
			TokenHash: tokenHash[:],
			UserID:    secondUser.ID,
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		err = repo.CreateUserWithSession(ctx, secondUser, secondSession)
		require.Error(t, err)

		var userCount int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE id = $1
		`, secondUser.ID).Scan(&userCount)
		require.NoError(t, err)
		assert.Zero(t, userCount)
	})

	t.Run("returns username already exists when username is duplicated", func(t *testing.T) {
		ctx, _, repo := setupAuthRepositoryTest(t)

		now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

		firstTokenHash := sha256.Sum256([]byte("first-session-token"))
		firstUser := domain.User{
			ID:           uuid.New(),
			Username:     "shared_dealer",
			PasswordHash: "first-password-hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		firstSession := domain.Session{
			TokenHash: firstTokenHash[:],
			UserID:    firstUser.ID,
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		err := repo.CreateUserWithSession(ctx, firstUser, firstSession)
		require.NoError(t, err)

		secondTokenHash := sha256.Sum256([]byte("second-session-token"))
		secondUser := domain.User{
			ID:           uuid.New(),
			Username:     firstUser.Username,
			PasswordHash: "second-password-hash",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		secondSession := domain.Session{
			TokenHash: secondTokenHash[:],
			UserID:    secondUser.ID,
			CreatedAt: now,
			ExpiresAt: now.Add(30 * 24 * time.Hour),
		}

		err = repo.CreateUserWithSession(ctx, secondUser, secondSession)

		require.ErrorIs(t, err, app.ErrUsernameAlreadyExists)
	})
}
