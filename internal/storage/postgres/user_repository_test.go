package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	user := domain.User{ID: uuid.New(), Login: "member", PasswordHash: "stored-hash", CreatedAt: time.Now().UTC()}
	t.Run("persists user", func(t *testing.T) {
		db := &stubTournamentDB{exec: func(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			assert.Equal(t, "INSERT INTO users (id, login, password_hash, created_at) VALUES ($1, $2, $3, $4)", compactSQL(query))
			assert.Equal(t, []any{user.ID, user.Login, user.PasswordHash, user.CreatedAt}, args)
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		}}
		require.NoError(t, (&UserRepository{pool: db}).Create(context.Background(), user))
	})
	t.Run("maps duplicate login and preserves cause", func(t *testing.T) {
		cause := &pgconn.PgError{Code: "23505", ConstraintName: "users_login_key"}
		db := &stubTournamentDB{exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, cause }}
		err := (&UserRepository{pool: db}).Create(context.Background(), user)
		require.ErrorIs(t, err, app.ErrLoginTaken)
		assert.ErrorIs(t, err, cause)
	})
	t.Run("does not call other SQL errors duplicate login", func(t *testing.T) {
		cause := &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}
		db := &stubTournamentDB{exec: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, cause }}
		err := (&UserRepository{pool: db}).Create(context.Background(), user)
		require.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, app.ErrLoginTaken)
	})
}

func TestUserRepository_Get(t *testing.T) {
	user := domain.User{ID: uuid.New(), Login: "member", PasswordHash: "stored-hash", CreatedAt: time.Now().UTC()}
	for _, byLogin := range []bool{false, true} {
		t.Run(map[bool]string{false: "by id", true: "by login"}[byLogin], func(t *testing.T) {
			db := &stubTournamentDB{queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
				if byLogin {
					assert.Contains(t, compactSQL(query), "WHERE login = $1")
					assert.Equal(t, []any{user.Login}, args)
				} else {
					assert.Contains(t, compactSQL(query), "WHERE id = $1")
					assert.Equal(t, []any{user.ID}, args)
				}
				return tournamentRowFunc(func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = user.ID
					*(dest[1].(*string)) = user.Login
					*(dest[2].(*string)) = user.PasswordHash
					*(dest[3].(*time.Time)) = user.CreatedAt
					return nil
				})
			}}
			repo := &UserRepository{pool: db}
			var got domain.User
			var err error
			if byLogin {
				got, err = repo.GetByLogin(context.Background(), user.Login)
			} else {
				got, err = repo.GetByID(context.Background(), user.ID)
			}
			require.NoError(t, err)
			assert.Equal(t, user, got)
		})
	}
	for _, cause := range []error{pgx.ErrNoRows, errors.New("SQL failure")} {
		t.Run(cause.Error(), func(t *testing.T) {
			db := &stubTournamentDB{queryRow: func(context.Context, string, ...any) pgx.Row {
				return tournamentRowFunc(func(...any) error { return cause })
			}}
			got, err := (&UserRepository{pool: db}).GetByLogin(context.Background(), "member")
			assert.Equal(t, domain.User{}, got)
			if errors.Is(cause, pgx.ErrNoRows) {
				require.ErrorIs(t, err, app.ErrUserNotFound)
			} else {
				require.ErrorIs(t, err, cause)
			}
		})
	}
}

func TestUserRepository_Integration(t *testing.T) {
	ctx, pool, _ := setupTournamentRepositoryTest(t)
	repo := NewUserRepository(pool)
	user := domain.User{ID: uuid.New(), Login: "member-" + uuid.NewString()[:16], PasswordHash: "stored-hash", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	require.NoError(t, repo.Create(ctx, user))
	byLogin, err := repo.GetByLogin(ctx, user.Login)
	require.NoError(t, err)
	assert.Equal(t, user, byLogin)
	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user, byID)
	duplicate := user
	duplicate.ID = uuid.New()
	require.ErrorIs(t, repo.Create(ctx, duplicate), app.ErrLoginTaken)
	_, err = repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, app.ErrUserNotFound)
}
