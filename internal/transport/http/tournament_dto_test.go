package http

import (
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTournamentRequest_ToServiceInput(t *testing.T) {
	t.Run("maps request to service input", func(t *testing.T) {
		req := CreateTournamentRequest{
			Name:                 "Friday Game",
			Date:                 time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC),
			Players:              []string{"A", "B", "C", "D"},
			BuyInAmount:          1000,
			Chips:                []ChipRequest{{Value: 25, Count: 80}},
			Style:                "standard",
			DurationMinutes:      120,
			LevelDurationMinutes: 20,
			Rebuy:                RebuyRequest{Allowed: true, MaxLevel: 3},
			Payout:               PayoutRequest{PayoutMode: "default"},
		}

		input, err := req.ToServiceInput()

		require.NoError(t, err)
		assert.Equal(t, req.Name, input.Name)
		assert.Equal(t, domain.StyleStandard, input.Style)
		assert.Equal(t, 120*time.Minute, input.Duration)
		assert.Equal(t, 20*time.Minute, input.LevelDuration)
		assert.Equal(t, []domain.ChipDenomination{{Value: 25, Count: 80}}, input.Chips)
		assert.Equal(t, service.PayoutModeDefault, input.PayoutMode)
	})

	t.Run("returns error when style is invalid", func(t *testing.T) {
		req := CreateTournamentRequest{Style: "fast"}

		input, err := req.ToServiceInput()

		require.Error(t, err)
		assert.Equal(t, service.CreateTournamentInput{}, input)
	})
}
