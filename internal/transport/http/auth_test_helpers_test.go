package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
)

var testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000071")

type fakeAuthService struct {
	register     func(context.Context, string, string, time.Time) (domain.User, error)
	login        func(context.Context, string, string, time.Time) (service.AuthResult, error)
	authenticate func(context.Context, string, time.Time) (uuid.UUID, error)
	logout       func(context.Context, string) error
	currentUser  func(context.Context, uuid.UUID) (domain.User, error)
}

func (s *fakeAuthService) Register(ctx context.Context, login, password string, now time.Time) (domain.User, error) {
	if s.register != nil {
		return s.register(ctx, login, password, now)
	}
	return domain.User{ID: testUserID, Login: "organizer", PasswordHash: "private-hash", CreatedAt: now}, nil
}

func (s *fakeAuthService) Login(ctx context.Context, login, password string, now time.Time) (service.AuthResult, error) {
	if s.login != nil {
		return s.login(ctx, login, password, now)
	}
	return service.AuthResult{UserID: testUserID, Login: "organizer", Token: "test-session", ExpiresAt: now.Add(30 * 24 * time.Hour)}, nil
}

func (s *fakeAuthService) Authenticate(ctx context.Context, token string, now time.Time) (uuid.UUID, error) {
	if s.authenticate != nil {
		return s.authenticate(ctx, token, now)
	}
	return testUserID, nil
}

func (s *fakeAuthService) Logout(ctx context.Context, token string) error {
	if s.logout != nil {
		return s.logout(ctx, token)
	}
	return nil
}

func (s *fakeAuthService) CurrentUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	if s.currentUser != nil {
		return s.currentUser(ctx, id)
	}
	return domain.User{ID: id, Login: "organizer", PasswordHash: "private-hash"}, nil
}

func newTournamentTestRouter(tournament *TournamentHandler) http.Handler {
	router := NewRouter(tournament, NewAuthHandler(&fakeAuthService{}, CookieOptions{}, config.DefaultRequestSafety()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "holdem_session", Value: "test-session"})
		router.ServeHTTP(w, r)
	})
}
