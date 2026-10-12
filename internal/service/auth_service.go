package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
)

type UserRepository interface {
	Create(context.Context, domain.User) error
	GetByLogin(context.Context, string) (domain.User, error)
	GetByID(context.Context, uuid.UUID) (domain.User, error)
}

type SessionStore interface {
	Create(context.Context, string, domain.Session) error
	Get(context.Context, string) (domain.Session, error)
	Delete(context.Context, string) error
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
}

type SessionTokenGenerator interface {
	Generate() (rawToken string, tokenHash string, err error)
	Hash(rawToken string) (string, error)
}

type AuthResult struct {
	UserID    uuid.UUID
	Login     string
	Token     string
	ExpiresAt time.Time
}

type AuthService struct {
	users     UserRepository
	sessions  SessionStore
	passwords PasswordHasher
	tokens    SessionTokenGenerator
	lifetime  time.Duration
}

func NewAuthService(users UserRepository, sessions SessionStore, passwords PasswordHasher, tokens SessionTokenGenerator, lifetime time.Duration) *AuthService {
	return &AuthService{users: users, sessions: sessions, passwords: passwords, tokens: tokens, lifetime: lifetime}
}

const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$MDEyMzQ1Njc4OWFiY2RlZg$i3a7WDpewjUyF3TF4Pumuqa3scxBvan2DGoIJRf0FIQ"

func (s *AuthService) Register(ctx context.Context, login, password string, now time.Time) (domain.User, error) {
	login, err := normalizeAuthLogin(login)
	if err != nil {
		return domain.User{}, err
	}
	if err := validateAuthPassword(password); err != nil {
		return domain.User{}, err
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash registration password: %w", err)
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate user id: %w", err)
	}
	user := domain.User{ID: id, Login: login, PasswordHash: hash, CreatedAt: now}
	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, login, password string, now time.Time) (AuthResult, error) {
	login, err := normalizeAuthLogin(login)
	if err != nil {
		return AuthResult{}, err
	}
	if err := validateAuthPassword(password); err != nil {
		return AuthResult{}, err
	}
	user, err := s.users.GetByLogin(ctx, login)
	missing := errors.Is(err, app.ErrUserNotFound)
	if err != nil && !missing {
		return AuthResult{}, fmt.Errorf("get login user: %w", err)
	}
	hash := user.PasswordHash
	if missing {
		hash = dummyPasswordHash
	}
	verified, err := s.passwords.Verify(password, hash)
	if err != nil {
		return AuthResult{}, fmt.Errorf("verify login password: %w", err)
	}
	if missing || !verified || user.ID == uuid.Nil {
		return AuthResult{}, app.ErrInvalidCredentials
	}
	token, tokenHash, err := s.tokens.Generate()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate login token: %w", err)
	}
	session := domain.Session{UserID: user.ID, CreatedAt: now, ExpiresAt: now.Add(s.lifetime)}
	if err := s.sessions.Create(ctx, tokenHash, session); err != nil {
		return AuthResult{}, fmt.Errorf("%w: create session: %w", app.ErrSessionUnavailable, err)
	}
	return AuthResult{UserID: user.ID, Login: user.Login, Token: token, ExpiresAt: session.ExpiresAt}, nil
}

func (s *AuthService) Authenticate(ctx context.Context, rawToken string, now time.Time) (uuid.UUID, error) {
	hash, err := s.tokens.Hash(rawToken)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: validate token: %w", app.ErrUnauthenticated, err)
	}
	session, err := s.sessions.Get(ctx, hash)
	if errors.Is(err, app.ErrSessionNotFound) {
		return uuid.Nil, fmt.Errorf("%w: %w", app.ErrUnauthenticated, err)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: get session: %w", app.ErrSessionUnavailable, err)
	}
	if session.UserID == uuid.Nil || now.IsZero() || session.CreatedAt.IsZero() || session.ExpiresAt.IsZero() || session.CreatedAt.After(now) || !session.ExpiresAt.After(session.CreatedAt) || !session.ExpiresAt.After(now) {
		return uuid.Nil, app.ErrUnauthenticated
	}
	return session.UserID, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	hash, err := s.tokens.Hash(rawToken)
	if err != nil {
		return nil
	}
	if err := s.sessions.Delete(ctx, hash); err != nil && !errors.Is(err, app.ErrSessionNotFound) {
		return fmt.Errorf("%w: delete session: %w", app.ErrSessionUnavailable, err)
	}
	return nil
}

func (s *AuthService) CurrentUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if id == uuid.Nil {
		return domain.User{}, app.ErrUnauthenticated
	}
	user, err := s.users.GetByID(ctx, id)
	if errors.Is(err, app.ErrUserNotFound) {
		return domain.User{}, fmt.Errorf("%w: %w", app.ErrUnauthenticated, err)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get current user: %w", err)
	}
	if user.ID != id {
		return domain.User{}, app.ErrUnauthenticated
	}
	return user, nil
}
