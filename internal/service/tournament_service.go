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

func (s *TournamentService) CreateTournament(ctx context.Context, input CreateTournamentInput) (uuid.UUID, error) {
	stack, err := domain.GenerateStackPlan(len(input.Players), input.Chips, input.Style)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: generate stack plan: %w", app.ErrValidation, err)
	}

	blinds, err := domain.GenerateBlinds(stack, input.Style, input.Duration, input.LevelDuration)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: generate blinds: %w", app.ErrValidation, err)
	}

	var payouts []domain.PayoutSpot

	switch input.PayoutMode {
	case PayoutModeCustom:
		payouts, err = domain.CustomPayouts(input.PayoutFixedBuyIns, len(input.Players))
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: build custom payouts: %w", app.ErrValidation, err)
		}
	case PayoutModeDefault:
		payouts, err = domain.DefaultPayouts(len(input.Players))
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: build default payouts: %w", app.ErrValidation, err)
		}
	default:
		return uuid.Nil, fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidPayoutMode)
	}

	tr, err := domain.NewTournament(input.Name,
		input.Date,
		input.Players,
		input.BuyInAmount,
		input.Chips,
		domain.RebuyRules{
			Allowed:  input.RebuyAllowed,
			MaxLevel: input.RebuyMaxLevel,
		},
		input.Duration,
		stack,
		blinds,
		payouts,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: build tournament: %w", app.ErrValidation, err)
	}

	id, err := s.repo.Create(ctx, tr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create tournament: %w", err)
	}

	return id, nil
}

func (s *TournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID, now time.Time) (*domain.Tournament, error) {
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}

	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get tournament: %w", err)
	}

	if tr.Status != domain.StatusRunning {
		return tr, nil
	}

	currentLvl := tr.CurrentLevel

	if err := tr.AdvanceLevelIfNeeded(now); err != nil {
		return nil, fmt.Errorf("advance blind level: %w", err)
	}

	if tr.CurrentLevel != currentLvl {
		if err := s.repo.Update(ctx, tr); err != nil {
			return nil, fmt.Errorf("update tournament: %w", err)
		}
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
	tr, err := s.GetTournamentByID(ctx, id, now)
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
	tr, err := s.GetTournamentByID(ctx, id, now)
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
	tr, err := s.GetTournamentByID(ctx, id, now)
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

func (s *TournamentService) FinishTournament(ctx context.Context, id uuid.UUID, now time.Time) error {
	tr, err := s.GetTournamentByID(ctx, id, now)
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
	if id == uuid.Nil {
		return app.ErrInvalidTournamentID
	}
	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get tournament: %w", err)
	}

	currentLvl := tr.CurrentLevel

	if err := tr.AdvanceLevelIfNeeded(now); err != nil {
		return fmt.Errorf("advance blind level: %w", err)
	}

	if currentLvl == tr.CurrentLevel {
		if err := tr.LevelUp(now); err != nil {
			return err
		}
	}

	if err := s.repo.Update(ctx, tr); err != nil {
		return fmt.Errorf("update tournament: %w", err)
	}

	return nil
}

func (s *TournamentService) AddRebuy(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidPlayerName)
	}

	tr, err := s.GetTournamentByID(ctx, id, now)
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

func (s *TournamentService) KnockoutPlayer(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error {
	playerName = strings.TrimSpace(playerName)
	if playerName == "" {
		return fmt.Errorf("%w: %w", app.ErrValidation, app.ErrInvalidPlayerName)
	}

	tr, err := s.GetTournamentByID(ctx, id, now)
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
