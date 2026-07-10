package service

import (
	"context"
	"fmt"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TournamentRepository interface {
	Create(ctx context.Context, tournament *domain.Tournament) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	Update(ctx context.Context, tournament *domain.Tournament) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type TournamentService struct {
	repo TournamentRepository
}

func NewTournamentService(repo TournamentRepository) *TournamentService {
	return &TournamentService{repo: repo}
}

func (s *TournamentService) CreateTournament(ctx context.Context, name string,
	date time.Time,
	players []string,
	buyInAmount int64,
	chips []domain.ChipDenomination,
	rebuyRules domain.RebuyRules,
	duration time.Duration,
	stack domain.StackPlan,
	blinds []domain.BlindLevel,
	payouts []domain.PayoutSpot,
) (uuid.UUID, error) {
	tr, err := domain.NewTournament(name, date, players, buyInAmount, chips, rebuyRules, duration, stack, blinds, payouts)
	if err != nil {
		return uuid.Nil, fmt.Errorf("build tournament: %w", err)
	}

	id, err := s.repo.Create(ctx, tr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create tournament: %w", err)
	}

	return id, nil
}

func (s *TournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}

	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get tournament: %w", err)
	}

	return tr, nil
}

func (s *TournamentService) DeleteTournament(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return app.ErrInvalidTournamentID
	}

	err := s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete tournament: %w", err)
	}
	return nil
}

func (s *TournamentService) StartTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.Start(now); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) PauseTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.Pause(now); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) ResumeTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.Resume(now); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) FinishTournament(ctx context.Context, id uuid.UUID) error {
	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.Finish(); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) LevelUpTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.LevelUp(now); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) AddRebuy(ctx context.Context, id uuid.UUID, playerName string) error {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return app.ErrInvalidPlayerName
	}

	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.AddRebuy(playerName); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) KnockoutPlayer(ctx context.Context, id uuid.UUID, playerName string) error {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return app.ErrInvalidPlayerName
	}

	tr, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return err
	}

	if err := tr.Knockout(playerName); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}
