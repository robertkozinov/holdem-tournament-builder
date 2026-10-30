package service

import (
	"context"
	"fmt"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
)

type StatisticsRepository interface {
	Leaderboard(context.Context, uuid.UUID, int, int) ([]domain.PlayerStatistics, error)
}

type StatisticsService struct{ repo StatisticsRepository }

func NewStatisticsService(repo StatisticsRepository) *StatisticsService {
	return &StatisticsService{repo: repo}
}

func (s *StatisticsService) Leaderboard(ctx context.Context, limit, offset int) ([]domain.PlayerStatistics, error) {
	owner, err := app.UserID(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, app.ErrValidation
	}
	items, err := s.repo.Leaderboard(ctx, owner, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list player statistics: %w", err)
	}
	return items, nil
}
