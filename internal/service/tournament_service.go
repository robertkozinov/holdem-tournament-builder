package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
)

type TournamentRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, tournament *domain.Tournament) (uuid.UUID, error)
	GetByID(ctx context.Context, ownerID, id uuid.UUID) (*domain.Tournament, error)
	Update(ctx context.Context, ownerID uuid.UUID, tournament *domain.Tournament) error
	UpdateLocked(ctx context.Context, ownerID, id uuid.UUID, change func(*domain.Tournament) (*domain.Tournament, error)) (*domain.Tournament, error)
	Delete(ctx context.Context, ownerID, id uuid.UUID) error
	List(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]*domain.Tournament, error)
}

type TournamentService struct{ repo TournamentRepository }

func NewTournamentService(repo TournamentRepository) *TournamentService {
	return &TournamentService{repo: repo}
}

func (s *TournamentService) CreateTournament(ctx context.Context, input CreateTournamentInput) (uuid.UUID, error) {
	ownerID, err := app.UserID(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	editable := SaveTournamentInput{Name: input.Name, Date: input.Date, Players: input.Players, SettlementMode: input.SettlementMode, ChipSet: input.ChipSet, Style: input.Style, Duration: input.Duration, LevelDuration: input.LevelDuration}
	tr, err := buildTournament(editable, input.BuyInAmount, true)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := s.repo.Create(ctx, ownerID, tr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create tournament: %w", err)
	}
	return id, nil
}

func (s *TournamentService) SaveTournament(ctx context.Context, id uuid.UUID, input SaveTournamentInput) (*domain.Tournament, error) {
	ownerID, err := app.UserID(ctx)
	if err != nil {
		return nil, err
	}
	original, err := s.GetTournamentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// A separate candidate leaves the loaded card intact on validation or storage failure.
	candidate, err := buildTournament(input, original.BuyInAmount, false)
	if err != nil {
		return nil, err
	}
	candidate.ID = id
	if err := s.repo.Update(ctx, ownerID, candidate); err != nil {
		return nil, fmt.Errorf("save tournament: %w", err)
	}
	return candidate, nil
}

func buildTournament(input SaveTournamentInput, buyIn int64, generateIDs bool) (*domain.Tournament, error) {
	players := slices.Clone(input.Players)
	for i := range players {
		players[i].Name = strings.TrimSpace(players[i].Name)
		if generateIDs && players[i].ID == uuid.Nil {
			players[i].ID = uuid.New()
		}
		players[i].Rebuys = slices.Clone(players[i].Rebuys)
		for j := range players[i].Rebuys {
			if generateIDs && players[i].Rebuys[j].ID == uuid.Nil {
				players[i].Rebuys[j].ID = uuid.New()
			}
		}
	}
	tr, err := domain.NewTournament(domain.Tournament{
		Name: strings.TrimSpace(input.Name), Date: input.Date, Players: players, BuyInAmount: buyIn,
		SettlementMode: input.SettlementMode, ChipSet: input.ChipSet, Style: input.Style,
		Duration: input.Duration, LevelDuration: input.LevelDuration, Results: input.Results,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: build tournament: %w", app.ErrValidation, err)
	}
	if tr.Duration/tr.LevelDuration > 1000 {
		return nil, fmt.Errorf("%w: %w", app.ErrValidation, domain.ErrTooManyBlindLevels)
	}
	stack, err := domain.GenerateStackPlan(len(tr.Players), tr.ChipSet, tr.Style)
	if err != nil {
		return nil, fmt.Errorf("%w: generate stack plan: %w", app.ErrValidation, err)
	}
	blinds, err := domain.GenerateBlinds(stack, tr.Style, tr.Duration, tr.LevelDuration)
	if err != nil {
		return nil, fmt.Errorf("%w: generate blinds: %w", app.ErrValidation, err)
	}
	tr.StartingStack = stack
	tr.BlindStructure = blinds
	return tr, nil
}

func (s *TournamentService) GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error) {
	ownerID, err := app.UserID(ctx)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}
	tr, err := s.repo.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, fmt.Errorf("get tournament: %w", err)
	}
	return tr, nil
}

func (s *TournamentService) DeleteTournament(ctx context.Context, id uuid.UUID) error {
	ownerID, err := app.UserID(ctx)
	if err != nil {
		return err
	}
	if id == uuid.Nil {
		return app.ErrInvalidTournamentID
	}
	if err := s.repo.Delete(ctx, ownerID, id); err != nil {
		return fmt.Errorf("delete tournament: %w", err)
	}
	return nil
}

func (s *TournamentService) ListTournaments(ctx context.Context, limit, offset int) ([]*domain.Tournament, error) {
	ownerID, err := app.UserID(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, fmt.Errorf("%w: limit must be 1..100 and offset non-negative", app.ErrValidation)
	}
	tournaments, err := s.repo.List(ctx, ownerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	return tournaments, nil
}
