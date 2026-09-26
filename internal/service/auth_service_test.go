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
	createUserWithSession struct {
		called  bool
		user    domain.User
		session domain.Session
		err     error
	}

	getUserByUsername struct {
		called   bool
		username string
		user     *domain.User
		err      error
	}

	createSession struct {
		called  bool
		session domain.Session
		err     error
	}

	getUserBySessionTokenHash struct {
		called    bool
		tokenHash []byte
		now       time.Time
		user      *domain.User
		err       error
	}

	deleteSession struct {
		called    bool
		tokenHash []byte
		err       error
	}
}

func (r *mockAuthRepository) CreateUserWithSession(ctx context.Context, user domain.User, session domain.Session) error {
	r.createUserWithSession.called = true
	r.createUserWithSession.user = user
	r.createUserWithSession.session = session
	return r.createUserWithSession.err
}

func (r *mockAuthRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	r.getUserByUsername.called = true
	r.getUserByUsername.username = username

	if r.getUserByUsername.err != nil {
		return nil, r.getUserByUsername.err
	}

	return r.getUserByUsername.user, nil
}

func (r *mockAuthRepository) CreateSession(ctx context.Context, session domain.Session) error {
	r.createSession.called = true
	r.createSession.session = session
	return r.createSession.err
}

func (r *mockAuthRepository) GetUserBySessionTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (*domain.User, error) {
	r.getUserBySessionTokenHash.called = true
	r.getUserBySessionTokenHash.tokenHash = tokenHash
	r.getUserBySessionTokenHash.now = now
	return r.getUserBySessionTokenHash.user, r.getUserBySessionTokenHash.err
}

func (r *mockAuthRepository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	r.deleteSession.called = true
	r.deleteSession.tokenHash = tokenHash
	return r.deleteSession.err
}

type mockPasswordHasher struct {
	called       bool
	password     string
	passwordHash string
	err          error

	verifyCalled       bool
	verifyPassword     string
	verifyPasswordHash string
	verifyResult       bool
	verifyErr          error
}

func (h *mockPasswordHasher) Hash(password string) (string, error) {
	h.called = true
	h.password = password
	return h.passwordHash, h.err
}

func (h *mockPasswordHasher) Verify(password string, encodedHash string) (bool, error) {
	h.verifyCalled = true
	h.verifyPassword = password
	h.verifyPasswordHash = encodedHash

	return h.verifyResult, h.verifyErr
}

type mockSessionTokenGenerator struct {
	generatorCalled    bool
	generatorRawToken  string
	generatorTokenHash []byte
	generatorErr       error

	hashCalled    bool
	hashRawToken  string
	hashTokenHash []byte
}

func (g *mockSessionTokenGenerator) Generate() (rawToken string, tokenHash []byte, err error) {
	g.generatorCalled = true
	return g.generatorRawToken, g.generatorTokenHash, g.generatorErr
}
func (g *mockSessionTokenGenerator) Hash(rawToken string) []byte {
	g.hashCalled = true
	g.hashRawToken = rawToken
	return g.hashTokenHash
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
			generatorRawToken:  "session-token",
			generatorTokenHash: tokenHash,
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

		assert.True(t, generator.generatorCalled)
		assert.True(t, repo.createUserWithSession.called)

		assert.NotEqual(t, uuid.Nil, repo.createUserWithSession.user.ID)
		assert.Equal(t, "tournament_admin", repo.createUserWithSession.user.Username)
		assert.Equal(t, "password-hash", repo.createUserWithSession.user.PasswordHash)
		assert.Equal(t, now, repo.createUserWithSession.user.CreatedAt)
		assert.Equal(t, now, repo.createUserWithSession.user.UpdatedAt)

		assert.Equal(t, repo.createUserWithSession.user.ID, repo.createUserWithSession.session.UserID)
		assert.Equal(t, tokenHash, repo.createUserWithSession.session.TokenHash)
		assert.Equal(t, now, repo.createUserWithSession.session.CreatedAt)
		assert.Equal(t, now.Add(30*24*time.Hour), repo.createUserWithSession.session.ExpiresAt)

		assert.Equal(t, repo.createUserWithSession.user.ID, result.UserID)
		assert.Equal(t, "tournament_admin", result.Username)
		assert.Equal(t, "session-token", result.Token)
		assert.Equal(t, repo.createUserWithSession.session.ExpiresAt, result.ExpiresAt)
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
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createUserWithSession.called)
	})

	t.Run("returns error when token generation fails", func(t *testing.T) {
		generateErr := errors.New("generate error")

		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{
			passwordHash: "password-hash",
		}
		generator := &mockSessionTokenGenerator{
			generatorErr: generateErr,
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
		assert.True(t, generator.generatorCalled)
		assert.False(t, repo.createUserWithSession.called)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		repoErr := errors.New("repository error")
		tokenHash := []byte{1, 2, 3, 4}

		repo := &mockAuthRepository{}
		repo.createUserWithSession.err = repoErr
		hasher := &mockPasswordHasher{
			passwordHash: "password-hash",
		}
		generator := &mockSessionTokenGenerator{
			generatorRawToken:  "session-token",
			generatorTokenHash: tokenHash,
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
		assert.True(t, generator.generatorCalled)
		assert.True(t, repo.createUserWithSession.called)
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
				assert.False(t, generator.generatorCalled)
				assert.False(t, repo.createUserWithSession.called)
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
				assert.False(t, generator.generatorCalled)
				assert.False(t, repo.createUserWithSession.called)
			})
		}
	})
}

func TestAuthService_Login(t *testing.T) {
	now := time.Date(2026, 7, 31, 20, 0, 0, 0, time.UTC)

	t.Run("login user", func(t *testing.T) {
		userID := uuid.New()
		tokenHash := []byte{1, 2, 3, 4}

		user := &domain.User{
			ID:           userID,
			Username:     "tournament_admin",
			PasswordHash: "password-hash",
		}

		repo := &mockAuthRepository{}
		repo.getUserByUsername.user = user

		hasher := &mockPasswordHasher{
			verifyResult: true,
		}

		generator := &mockSessionTokenGenerator{
			generatorRawToken:  "session-token",
			generatorTokenHash: tokenHash,
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"  Tournament_Admin ",
			"RiverCard7!",
			now,
		)

		require.NoError(t, err)

		assert.True(t, repo.getUserByUsername.called)
		assert.Equal(t, "tournament_admin", repo.getUserByUsername.username)

		assert.True(t, hasher.verifyCalled)
		assert.Equal(t, "RiverCard7!", hasher.verifyPassword)
		assert.Equal(t, "password-hash", hasher.verifyPasswordHash)

		assert.True(t, generator.generatorCalled)

		assert.True(t, repo.createSession.called)
		assert.Equal(t, tokenHash, repo.createSession.session.TokenHash)
		assert.Equal(t, userID, repo.createSession.session.UserID)
		assert.Equal(t, now, repo.createSession.session.CreatedAt)
		assert.Equal(t, now.Add(sessionLifetime), repo.createSession.session.ExpiresAt)

		assert.Equal(t, AuthResult{
			UserID:    userID,
			Username:  "tournament_admin",
			Token:     "session-token",
			ExpiresAt: now.Add(sessionLifetime),
		}, result)

		assert.False(t, hasher.called)
	})

	t.Run("returns invalid credentials when password is incorrect", func(t *testing.T) {
		user := &domain.User{
			ID:           uuid.New(),
			Username:     "tournament_admin",
			PasswordHash: "password-hash",
		}

		repo := &mockAuthRepository{}
		repo.getUserByUsername.user = user

		hasher := &mockPasswordHasher{
			verifyResult: false,
		}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"WrongPassword7!",
			now,
		)

		require.ErrorIs(t, err, app.ErrInvalidCredentials)
		assert.Equal(t, AuthResult{}, result)

		assert.True(t, repo.getUserByUsername.called)
		assert.True(t, hasher.verifyCalled)
		assert.Equal(t, "WrongPassword7!", hasher.verifyPassword)
		assert.Equal(t, "password-hash", hasher.verifyPasswordHash)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})

	t.Run("returns invalid credentials when user does not exist", func(t *testing.T) {
		repo := &mockAuthRepository{}
		repo.getUserByUsername.err = app.ErrUserNotFound

		hasher := &mockPasswordHasher{}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"  Missing_User ",
			"RiverCard7!",
			now,
		)

		assert.Equal(t, AuthResult{}, result)
		assert.True(t, repo.getUserByUsername.called)
		assert.Equal(t, "missing_user", repo.getUserByUsername.username)
		assert.False(t, hasher.verifyCalled)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
		require.ErrorIs(t, err, app.ErrInvalidCredentials)
	})

	t.Run("returns error when get user fails", func(t *testing.T) {
		repoErr := errors.New("repository error")

		repo := &mockAuthRepository{}
		repo.getUserByUsername.err = repoErr

		hasher := &mockPasswordHasher{}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, repoErr)
		assert.Equal(t, AuthResult{}, result)
		assert.True(t, repo.getUserByUsername.called)
		assert.False(t, hasher.verifyCalled)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})

	t.Run("returns error when password verification fails", func(t *testing.T) {
		verifyErr := errors.New("verify error")

		user := &domain.User{
			ID:           uuid.New(),
			Username:     "tournament_admin",
			PasswordHash: "password-hash",
		}

		repo := &mockAuthRepository{}
		repo.getUserByUsername.user = user

		hasher := &mockPasswordHasher{
			verifyErr: verifyErr,
		}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, verifyErr)
		assert.Equal(t, AuthResult{}, result)
		assert.True(t, repo.getUserByUsername.called)
		assert.True(t, hasher.verifyCalled)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})

	t.Run("returns error when token generation fails", func(t *testing.T) {
		generatorErr := errors.New("generator error")

		user := &domain.User{
			ID:           uuid.New(),
			Username:     "tournament_admin",
			PasswordHash: "password-hash",
		}

		repo := &mockAuthRepository{}
		repo.getUserByUsername.user = user

		hasher := &mockPasswordHasher{
			verifyResult: true,
		}
		generator := &mockSessionTokenGenerator{
			generatorErr: generatorErr,
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, generatorErr)
		assert.Equal(t, AuthResult{}, result)
		assert.True(t, repo.getUserByUsername.called)
		assert.True(t, hasher.verifyCalled)
		assert.True(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})

	t.Run("returns error when create session fails", func(t *testing.T) {
		repoErr := errors.New("repository error")

		user := &domain.User{
			ID:           uuid.New(),
			Username:     "tournament_admin",
			PasswordHash: "password-hash",
		}

		repo := &mockAuthRepository{}
		repo.getUserByUsername.user = user
		repo.createSession.err = repoErr

		hasher := &mockPasswordHasher{
			verifyResult: true,
		}
		generator := &mockSessionTokenGenerator{
			generatorRawToken:  "session-token",
			generatorTokenHash: []byte{1, 2, 3, 4},
		}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, repoErr)
		assert.Equal(t, AuthResult{}, result)
		assert.True(t, repo.getUserByUsername.called)
		assert.True(t, hasher.verifyCalled)
		assert.True(t, generator.generatorCalled)
		assert.True(t, repo.createSession.called)
	})

	t.Run("returns invalid credentials when username is invalid", func(t *testing.T) {
		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"x",
			"RiverCard7!",
			now,
		)

		require.ErrorIs(t, err, app.ErrInvalidCredentials)
		assert.Equal(t, AuthResult{}, result)
		assert.False(t, repo.getUserByUsername.called)
		assert.False(t, hasher.verifyCalled)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})

	t.Run("returns invalid credentials when password is invalid", func(t *testing.T) {
		repo := &mockAuthRepository{}
		hasher := &mockPasswordHasher{}
		generator := &mockSessionTokenGenerator{}

		service := NewAuthService(repo, hasher, generator)

		result, err := service.Login(
			context.Background(),
			"tournament_admin",
			"short",
			now,
		)

		require.ErrorIs(t, err, app.ErrInvalidCredentials)
		assert.Equal(t, AuthResult{}, result)
		assert.False(t, repo.getUserByUsername.called)
		assert.False(t, hasher.verifyCalled)
		assert.False(t, generator.generatorCalled)
		assert.False(t, repo.createSession.called)
	})
}

func TestAuthService_Authenticate(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	t.Run("returns user for active session", func(t *testing.T) {
		user := &domain.User{ID: uuid.New(), Username: "table_host"}
		repo := &mockAuthRepository{}
		repo.getUserBySessionTokenHash.user = user
		generator := &mockSessionTokenGenerator{hashTokenHash: []byte{1, 2, 3}}
		s := NewAuthService(repo, &mockPasswordHasher{}, generator)

		got, err := s.Authenticate(context.Background(), "raw-token", now)

		require.NoError(t, err)
		assert.Same(t, user, got)
		assert.True(t, generator.hashCalled)
		assert.Equal(t, "raw-token", generator.hashRawToken)
		assert.True(t, repo.getUserBySessionTokenHash.called)
		assert.Equal(t, []byte{1, 2, 3}, repo.getUserBySessionTokenHash.tokenHash)
		assert.Equal(t, now, repo.getUserBySessionTokenHash.now)
	})

	t.Run("rejects missing token", func(t *testing.T) {
		repo := &mockAuthRepository{}
		generator := &mockSessionTokenGenerator{}
		s := NewAuthService(repo, &mockPasswordHasher{}, generator)

		got, err := s.Authenticate(context.Background(), "", now)

		require.ErrorIs(t, err, app.ErrUnauthenticated)
		assert.Nil(t, got)
		assert.False(t, generator.hashCalled)
		assert.False(t, repo.getUserBySessionTokenHash.called)
	})

	t.Run("rejects unknown or expired session", func(t *testing.T) {
		repo := &mockAuthRepository{}
		repo.getUserBySessionTokenHash.err = app.ErrUnauthenticated
		s := NewAuthService(repo, &mockPasswordHasher{}, &mockSessionTokenGenerator{})

		got, err := s.Authenticate(context.Background(), "invalid-token", now)

		require.ErrorIs(t, err, app.ErrUnauthenticated)
		assert.Nil(t, got)
	})

	t.Run("preserves repository error", func(t *testing.T) {
		repoErr := errors.New("database unavailable")
		repo := &mockAuthRepository{}
		repo.getUserBySessionTokenHash.err = repoErr
		s := NewAuthService(repo, &mockPasswordHasher{}, &mockSessionTokenGenerator{})

		got, err := s.Authenticate(context.Background(), "raw-token", now)

		require.ErrorIs(t, err, repoErr)
		assert.Nil(t, got)
	})
}

func TestAuthService_Logout(t *testing.T) {
	t.Run("deletes session by token hash", func(t *testing.T) {
		repo := &mockAuthRepository{}
		generator := &mockSessionTokenGenerator{hashTokenHash: []byte{4, 5, 6}}
		s := NewAuthService(repo, &mockPasswordHasher{}, generator)

		err := s.Logout(context.Background(), "raw-token")

		require.NoError(t, err)
		assert.Equal(t, "raw-token", generator.hashRawToken)
		assert.True(t, repo.deleteSession.called)
		assert.Equal(t, []byte{4, 5, 6}, repo.deleteSession.tokenHash)
	})

	t.Run("missing token is already logged out", func(t *testing.T) {
		repo := &mockAuthRepository{}
		generator := &mockSessionTokenGenerator{}
		s := NewAuthService(repo, &mockPasswordHasher{}, generator)

		err := s.Logout(context.Background(), "")

		require.NoError(t, err)
		assert.False(t, generator.hashCalled)
		assert.False(t, repo.deleteSession.called)
	})

	t.Run("preserves repository error", func(t *testing.T) {
		repoErr := errors.New("database unavailable")
		repo := &mockAuthRepository{}
		repo.deleteSession.err = repoErr
		s := NewAuthService(repo, &mockPasswordHasher{}, &mockSessionTokenGenerator{})

		err := s.Logout(context.Background(), "raw-token")

		require.ErrorIs(t, err, repoErr)
	})
}
