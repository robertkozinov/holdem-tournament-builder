package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
)

type PlayerRepository interface {
	Create(context.Context, uuid.UUID, domain.Player) (domain.Player, error)
	List(context.Context, uuid.UUID, string, int, int) ([]domain.Player, error)
	Rename(context.Context, uuid.UUID, uuid.UUID, string, string, time.Time) (domain.Player, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, time.Time) error
}

type PlayerService struct{ repo PlayerRepository }

func NewPlayerService(repo PlayerRepository) *PlayerService { return &PlayerService{repo: repo} }

func (s *PlayerService) CreatePlayer(ctx context.Context, name string, now time.Time) (domain.Player, error) {
	owner, err := app.UserID(ctx)
	if err != nil {
		return domain.Player{}, err
	}
	display, normalized, err := domain.NormalizePlayerName(name)
	if err != nil {
		return domain.Player{}, fmt.Errorf("%w: %w", app.ErrValidation, err)
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return domain.Player{}, fmt.Errorf("generate player id: %w", err)
	}
	player, err := s.repo.Create(ctx, owner, domain.Player{ID: id, Name: display, NormalizedName: normalized, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return domain.Player{}, fmt.Errorf("create player: %w", err)
	}
	return player, nil
}

func (s *PlayerService) ListPlayers(ctx context.Context, query string, limit, offset int) ([]domain.Player, error) {
	owner, err := app.UserID(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, app.ErrValidation
	}
	normalized := ""
	if strings.TrimSpace(query) != "" {
		_, normalized, err = domain.NormalizePlayerName(query)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", app.ErrValidation, err)
		}
	}
	players, err := s.repo.List(ctx, owner, normalized, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}
	return players, nil
}

func (s *PlayerService) RenamePlayer(ctx context.Context, id uuid.UUID, name string, now time.Time) (domain.Player, error) {
	owner, err := app.UserID(ctx)
	if err != nil {
		return domain.Player{}, err
	}
	if id == uuid.Nil {
		return domain.Player{}, app.ErrInvalidPlayerID
	}
	display, normalized, err := domain.NormalizePlayerName(name)
	if err != nil {
		return domain.Player{}, fmt.Errorf("%w: %w", app.ErrValidation, err)
	}
	player, err := s.repo.Rename(ctx, owner, id, display, normalized, now)
	if err != nil {
		return domain.Player{}, fmt.Errorf("rename player: %w", err)
	}
	return player, nil
}

func (s *PlayerService) DeletePlayer(ctx context.Context, id uuid.UUID, now time.Time) error {
	owner, err := app.UserID(ctx)
	if err != nil {
		return err
	}
	if id == uuid.Nil {
		return app.ErrInvalidPlayerID
	}
	if err := s.repo.Delete(ctx, owner, id, now); err != nil {
		return fmt.Errorf("delete player: %w", err)
	}
	return nil
}
