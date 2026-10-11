package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
)

type PatchTournamentPlayerInput struct {
	BuyInPaid *bool
	Rebuys    *[]domain.Rebuy
}

func (s *TournamentService) PatchTournamentPlayer(ctx context.Context, tournamentID, playerID uuid.UUID, input PatchTournamentPlayerInput) (*domain.Tournament, error) {
	if tournamentID == uuid.Nil {
		return nil, app.ErrInvalidTournamentID
	}
	if playerID == uuid.Nil {
		return nil, fmt.Errorf("%w: %w", app.ErrValidation, domain.ErrInvalidPlayerID)
	}
	if input.BuyInPaid == nil && input.Rebuys == nil {
		return nil, fmt.Errorf("%w: at least one player field is required", app.ErrValidation)
	}
	updated, err := s.repo.UpdateLocked(ctx, tournamentID, func(original *domain.Tournament) (*domain.Tournament, error) {
		index := slices.IndexFunc(original.Players, func(player domain.TournamentPlayer) bool {
			return player.ID == playerID
		})
		if index < 0 {
			return nil, domain.ErrPlayerNotFound
		}
		candidate := *original
		candidate.Players = slices.Clone(original.Players)
		if input.BuyInPaid != nil {
			candidate.Players[index].BuyInPaid = *input.BuyInPaid
		}
		if input.Rebuys != nil {
			candidate.Players[index].Rebuys = *input.Rebuys
		}
		updated, err := domain.NewTournament(candidate)
		if err != nil {
			return nil, fmt.Errorf("%w: patch tournament player: %w", app.ErrValidation, err)
		}
		return updated, nil
	})
	if err != nil {
		if errors.Is(err, app.ErrValidation) {
			return nil, err
		}
		return nil, fmt.Errorf("patch tournament player: %w", err)
	}
	return updated, nil
}
