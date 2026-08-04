package service

import (
	"context"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const sessionLifetime = 30 * 24 * time.Hour

type AuthRepository interface {
	CreateUserWithSession(ctx context.Context, user domain.User, session domain.Session) error
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	CreateSession(ctx context.Context, session domain.Session) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, encodedHash string) (bool, error)
}

type SessionTokenGenerator interface {
	Generate() (rawToken string, tokenHash []byte, err error)
	Hash(rawToken string) []byte
}

type AuthService struct {
	repo      AuthRepository
	hasher    PasswordHasher
	generator SessionTokenGenerator
}

func NewAuthService(repo AuthRepository, hasher PasswordHasher, generator SessionTokenGenerator) *AuthService {
	return &AuthService{
		repo:      repo,
		hasher:    hasher,
		generator: generator,
	}
}

type AuthResult struct {
	UserID    uuid.UUID
	Username  string
	Token     string
	ExpiresAt time.Time
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return app.ErrInvalidUsername
	}

	for _, char := range username {
		if char >= 'a' && char <= 'z' ||
			char >= '0' && char <= '9' ||
			char == '_' {
			continue
		}

		return app.ErrInvalidUsername
	}

	return nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 6 || length > 128 {
		return app.ErrInvalidPassword
	}

	for _, char := range password {
		if unicode.IsSpace(char) {
			return app.ErrInvalidPassword
		}
	}

	return nil
}

func (s *AuthService) Register(ctx context.Context, username, password string, now time.Time) (AuthResult, error) {
	normalizedUsername := normalizeUsername(username)
	if err := validateUsername(normalizedUsername); err != nil {
		return AuthResult{}, fmt.Errorf("%w: %w", app.ErrValidation, err)
	}
	if err := validatePassword(password); err != nil {
		return AuthResult{}, fmt.Errorf("%w: %w", app.ErrValidation, err)
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	rawToken, tokenHash, err := s.generator.Generate()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate session token: %w", err)
	}

	expiresAt := now.Add(sessionLifetime)

	user := domain.User{
		ID:           uuid.New(),
		Username:     normalizedUsername,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	session := domain.Session{
		TokenHash: tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := s.repo.CreateUserWithSession(ctx, user, session); err != nil {
		return AuthResult{}, fmt.Errorf("create user with session: %w", err)
	}

	res := AuthResult{
		UserID:    user.ID,
		Username:  user.Username,
		Token:     rawToken,
		ExpiresAt: expiresAt,
	}

	return res, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string, now time.Time) (AuthResult, error) {
	normalizedUsername := normalizeUsername(username)
	if err := validateUsername(normalizedUsername); err != nil {
		return AuthResult{}, app.ErrInvalidCredentials
	}
	if err := validatePassword(password); err != nil {
		return AuthResult{}, app.ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByUsername(ctx, normalizedUsername)
	if err != nil {
		if errors.Is(err, app.ErrUserNotFound) {
			return AuthResult{}, app.ErrInvalidCredentials
		}
		return AuthResult{}, fmt.Errorf("get user by username: %w", err)
	}

	valid, err := s.hasher.Verify(password, user.PasswordHash)
	if err != nil {
		return AuthResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return AuthResult{}, app.ErrInvalidCredentials
	}

	rawToken, tokenHash, err := s.generator.Generate()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate session token: %w", err)
	}

	expiresAt := now.Add(sessionLifetime)

	session := domain.Session{
		TokenHash: tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return AuthResult{}, fmt.Errorf("create session: %w", err)
	}

	result := AuthResult{
		UserID:    user.ID,
		Username:  user.Username,
		Token:     rawToken,
		ExpiresAt: expiresAt,
	}

	return result, nil
}
