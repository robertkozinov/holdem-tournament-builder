package service

import (
	"context"
	"testing"

	"holdem-tournament-builder/internal/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTournamentService_RequiresAuthentication(t *testing.T) {
	tests := []struct {
		name string
		run  func(*TournamentService) error
	}{
		{"create", func(s *TournamentService) error {
			_, err := s.CreateTournament(context.Background(), validCreateTournamentInput())
			return err
		}},
		{"get", func(s *TournamentService) error {
			_, err := s.GetTournamentByID(context.Background(), testTournamentID)
			return err
		}},
		{"save", func(s *TournamentService) error {
			_, err := s.SaveTournament(context.Background(), testTournamentID, validSaveTournamentInput())
			return err
		}},
		{"patch", func(s *TournamentService) error {
			paid := true
			_, err := s.PatchTournamentPlayer(context.Background(), testTournamentID, validSaveTournamentInput().Players[0].ID, PatchTournamentPlayerInput{BuyInPaid: &paid})
			return err
		}},
		{"delete", func(s *TournamentService) error { return s.DeleteTournament(context.Background(), testTournamentID) }},
		{"list", func(s *TournamentService) error { _, err := s.ListTournaments(context.Background(), 50, 0); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockTournamentRepository{getTournament: serviceTournamentFixture(t, validSaveTournamentInput())}
			err := tt.run(NewTournamentService(repo))
			require.ErrorIs(t, err, app.ErrUnauthenticated)
			assert.False(t, repo.createCalled || repo.getCalled || repo.updateCalled || repo.lockedCalled || repo.deleteCalled || repo.listCalled)
		})
	}
}
