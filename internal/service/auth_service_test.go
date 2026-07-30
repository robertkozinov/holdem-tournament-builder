package service

import (
	"context"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuthRepository struct {
	called  bool
	user    domain.User
	session domain.Session
	err     error
}

func (r *mockAuthRepository) CreateUserWithSession(ctx context.Context, user domain.User, session domain.Session) error {
	r.called = true
	r.user = user
	r.session = session
	return r.err
}

type mockPasswordHasher struct {
	called       bool
	password     string
	passwordHash string
	err          error
}

func (h *mockPasswordHasher) Hash(password string) (string, error) {
	h.called = true
	h.password = password
	return h.passwordHash, h.err
}

func (h *mockPasswordHasher) Verify(password string, encodedHash string) (bool, error) {
	return false, nil
}

type mockSessionTokenGenerator struct {
	called    bool
	rawToken  string
	tokenHash []byte
	err       error
}

func (g *mockSessionTokenGenerator) Generate() (rawToken string, tokenHash []byte, err error) {
	g.called = true
	return g.rawToken, g.tokenHash, g.err
}

func TestAuthService_Register(t *testing.T) {
	now := time.Date(2026, 7, 29, 20, 0, 0, 0, time.UTC)
	t.Run("registers user", func(t *testing.T) {
		tokenHash := []byte{1, 2, 3, 4}

		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{
			passwordHash: "password-hash",
		}
		generator := &mockSessionTokenGenerator{
			rawToken:  "session-token",
			tokenHash: tokenHash,
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Register(
			context.Background(),
			"  Tournament_Admin ",
			"RiverCard7!",
			now,
		)

		require.NoError(t, err)

		assert.True(t, hasher.called)
		assert.Equal(t, "RiverCard7!", hasher.password)

		assert.True(t, generator.called)
		assert.True(t, repo.called)

		assert.NotEqual(t, uuid.Nil, repo.user.ID)
		assert.Equal(t, "tournament_admin", repo.user.Username)
		assert.Equal(t, "password-hash", repo.user.PasswordHash)
		assert.Equal(t, now, repo.user.CreatedAt)
		assert.Equal(t, now, repo.user.UpdatedAt)

		assert.Equal(t, repo.user.ID, repo.session.UserID)
		assert.Equal(t, tokenHash, repo.session.TokenHash)
		assert.Equal(t, now, repo.session.CreatedAt)
		assert.Equal(t, now.Add(30*24*time.Hour), repo.session.ExpiresAt)

		assert.Equal(t, repo.user.ID, result.UserID)
		assert.Equal(t, "tournament_admin", result.Username)
		assert.Equal(t, "session-token", result.Token)
		assert.Equal(t, repo.session.ExpiresAt, result.ExpiresAt)
	})

	t.Run("returns error when password hashing fails", func(t *testing.T) {
		hashErr := errors.New("hash error")

		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{err: hashErr}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Register(
			context.Background(),
			"Tournament_Admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, hashErr)
		assert.Contains(t, err.Error(), "hash password")
		assert.Equal(t, AuthResult{}, result)

		assert.True(t, hasher.called)
		assert.Equal(t, "RiverCard7!", hasher.password)
		assert.False(t, generator.called)
		assert.False(t, repo.called)
	})

	t.Run("returns error when token generation fails", func(t *testing.T) {
		generateErr := errors.New("generate error")

		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{
			passwordHash: "password-hash",
		}
		generator := &mockSessionTokenGenerator{
			err: generateErr,
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Register(
			context.Background(),
			"Tournament_Admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, generateErr)
		assert.Contains(t, err.Error(), "generate session token")
		assert.Equal(t, AuthResult{}, result)

		assert.True(t, hasher.called)
		assert.True(t, generator.called)
		assert.False(t, repo.called)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		repoErr := errors.New("repository error")
		tokenHash := []byte{1, 2, 3, 4}

		repo := &mockAuthRepository{
			err: repoErr,
		}
		hasher := &mockPasswordHasher{
			passwordHash: "password-hash",
		}
		generator := &mockSessionTokenGenerator{
			rawToken:  "session-token",
			tokenHash: tokenHash,
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Register(
			context.Background(),
			"Tournament_Admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, repoErr)
		assert.Contains(t, err.Error(), "create user with session")
		assert.Equal(t, AuthResult{}, result)

		assert.True(t, hasher.called)
		assert.True(t, generator.called)
		assert.True(t, repo.called)
	})

	t.Run("returns error when username is invalid", func(t *testing.T) {
		tests := []struct {
			name     string
			username string
		}{
			{
				name:     "username is empty",
				username: "",
			},
			{
				name:     "username is too short",
				username: "ab",
			},
			{
				name:     "username is too long",
				username: strings.Repeat("a", 33),
			},
			{
				name:     "username contains space",
				username: "user name",
			},
			{
				name:     "username contains hyphen",
				username: "user-name",
			},
			{
				name:     "username contains non ASCII characters",
				username: "пользователь",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				repo := &mockAuthRepository{}
				hasher := &mockPasswordHasher{}
				generator := &mockSessionTokenGenerator{}

				service := NewAuthService(repo, hasher, generator)

				result, err := service.Register(
					context.Background(),
					test.username,
					"RiverCard7!",
					now,
				)

				require.ErrorIs(t, err, app.ErrValidation)
				require.ErrorIs(t, err, app.ErrInvalidUsername)
				assert.Equal(t, AuthResult{}, result)

				assert.False(t, hasher.called)
				assert.False(t, generator.called)
				assert.False(t, repo.called)
			})
		}
	})

	t.Run("returns error when password is invalid", func(t *testing.T) {
		tests := []struct {
			name     string
			password string
		}{
			{
				name:     "password is empty",
				password: "",
			},
			{
				name:     "password is too short",
				password: "Ab1!x",
			},
			{
				name:     "password is too long",
				password: strings.Repeat("a", 129),
			},
			{
				name:     "password contains leading space",
				password: " RiverCard7!",
			},
			{
				name:     "password contains internal space",
				password: "River Card7!",
			},
			{
				name:     "password contains tab",
				password: "River\tCard7!",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				repo := &mockAuthRepository{}
				hasher := &mockPasswordHasher{}
				generator := &mockSessionTokenGenerator{}

				service := NewAuthService(repo, hasher, generator)

				result, err := service.Register(
					context.Background(),
					"tournament_admin",
					test.password,
					now,
				)

				require.ErrorIs(t, err, app.ErrValidation)
				require.ErrorIs(t, err, app.ErrInvalidPassword)
				assert.Equal(t, AuthResult{}, result)

				assert.False(t, hasher.called)
				assert.False(t, generator.called)
				assert.False(t, repo.called)
			})
		}
	})
}
