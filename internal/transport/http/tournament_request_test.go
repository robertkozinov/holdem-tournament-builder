package http

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"math"
	"testing"
	"time"
)

func settingsRequestFixture() TournamentSettingsRequest {
	return TournamentSettingsRequest{
		Name: "Friday Game", Date: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
		Players:        []PlayerRequest{{ID: uuid.New(), Name: "A", BuyInPaid: true, Rebuys: []RebuyRequest{{ID: uuid.New(), Paid: false}}}, {ID: uuid.New(), Name: "B"}},
		SettlementMode: "p2p", ChipSet: []ChipRequest{{Value: 25, Count: 80}},
		Style: "standard", DurationMinutes: 120, LevelDurationMinutes: 20,
	}
}
func TestCreateTournamentRequest_ToServiceInput(t *testing.T) {
	t.Run("maps settings including separate payments", func(t *testing.T) {
		req := CreateTournamentRequest{TournamentSettingsRequest: settingsRequestFixture(), BuyInAmount: 1000}
		in, err := req.ToServiceInput()
		require.NoError(t, err)
		assert.Equal(t, req.Name, in.Name)
		assert.Equal(t, req.Date, in.Date)
		assert.Equal(t, int64(1000), in.BuyInAmount)
		assert.Equal(t, domain.StyleStandard, in.Style)
		assert.Equal(t, domain.SettlementP2P, in.SettlementMode)
		assert.Equal(t, 120*time.Minute, in.Duration)
		assert.Equal(t, 20*time.Minute, in.LevelDuration)
		assert.Equal(t, []domain.ChipDenomination{{Value: 25, Count: 80}}, in.ChipSet)
		assert.Equal(t, req.Players[0].ID, in.Players[0].ID)
		assert.True(t, in.Players[0].BuyInPaid)
		assert.Equal(t, req.Players[0].Rebuys[0].ID, in.Players[0].Rebuys[0].ID)
		assert.False(t, in.Players[0].Rebuys[0].Paid)
	})
	tests := []struct {
		name   string
		change func(*TournamentSettingsRequest)
	}{
		{"invalid style", func(r *TournamentSettingsRequest) { r.Style = "fast" }},
		{"invalid mode", func(r *TournamentSettingsRequest) { r.SettlementMode = "cash" }},
		{"zero duration", func(r *TournamentSettingsRequest) { r.DurationMinutes = 0 }},
		{"negative level duration", func(r *TournamentSettingsRequest) { r.LevelDurationMinutes = -1 }},
		{"duration overflow", func(r *TournamentSettingsRequest) { r.DurationMinutes = math.MaxInt }},
		{"level duration overflow", func(r *TournamentSettingsRequest) { r.LevelDurationMinutes = math.MaxInt }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := settingsRequestFixture()
			tt.change(&r)
			in, err := (CreateTournamentRequest{TournamentSettingsRequest: r}).ToServiceInput()
			require.Error(t, err)
			assert.Equal(t, service.CreateTournamentInput{}, in)
		})
	}
}
func TestSaveTournamentRequest_ToServiceInput(t *testing.T) {
	t.Run("maps manual split and ordinary place", func(t *testing.T) {
		r := settingsRequestFixture()
		place := 2
		req := SaveTournamentRequest{TournamentSettingsRequest: r, Results: []ResultRequest{{PlayerID: r.Players[0].ID, IsSplit: true, Prize: 2000}, {PlayerID: r.Players[1].ID, Place: &place, Prize: 1000}}}
		in, err := req.ToServiceInput()
		require.NoError(t, err)
		require.Len(t, in.Results, 2)
		assert.Equal(t, r.Players[0].ID, in.Results[0].PlayerID)
		assert.Nil(t, in.Results[0].Place)
		assert.Equal(t, int64(2000), in.Results[0].Prize)
		assert.Equal(t, 2, *in.Results[1].Place)
	})
	t.Run("split cannot have place", func(t *testing.T) {
		r := settingsRequestFixture()
		place := 1
		_, err := (SaveTournamentRequest{TournamentSettingsRequest: r, Results: []ResultRequest{{PlayerID: r.Players[0].ID, IsSplit: true, Place: &place, Prize: 3000}}}).ToServiceInput()
		require.Error(t, err)
	})
	t.Run("ordinary prize requires place", func(t *testing.T) {
		r := settingsRequestFixture()
		_, err := (SaveTournamentRequest{TournamentSettingsRequest: r, Results: []ResultRequest{{PlayerID: r.Players[0].ID, Prize: 3000}}}).ToServiceInput()
		require.Error(t, err)
	})
}
