package domain

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCalculatePayouts(t *testing.T) {
	tests := []struct {
		name            string
		players, rebuys int
		want            []PayoutResult
	}{
		{"winner takes all", 4, 0, []PayoutResult{{Place: 1, Value: 4000}}},
		{"two places", 5, 0, []PayoutResult{{Place: 1, Value: 4000}, {Place: 2, Value: 1000}}},
		{"three places", 8, 0, []PayoutResult{{Place: 1, Value: 5000}, {Place: 2, Value: 2000}, {Place: 3, Value: 1000}}},
		{"rebuys increase remainder", 5, 2, []PayoutResult{{Place: 1, Value: 6000}, {Place: 2, Value: 1000}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Tournament{BuyInAmount: 1000}
			for range tt.players {
				tr.Players = append(tr.Players, TournamentPlayer{ID: uuid.New()})
			}
			for range tt.rebuys {
				tr.Players[0].Rebuys = append(tr.Players[0].Rebuys, Rebuy{ID: uuid.New()})
			}
			got, err := tr.CalculatePayouts()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, tr.Results)
		})
	}
	t.Run("invalid number of players", func(t *testing.T) { _, err := (&Tournament{}).CalculatePayouts(); require.Error(t, err) })
}
func TestDefaultPayouts(t *testing.T) {
	tests := []struct {
		name    string
		players int
		want    []PayoutSpot
	}{
		{
			name:    "4 players",
			players: 4,
			want:    []PayoutSpot{{Place: 1, Kind: PayoutRemainder}},
		},
		{
			name:    "5 players",
			players: 5,
			want: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 1},
			},
		},
		{
			name:    "8 players",
			players: 8,
			want: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 2},
				{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DefaultPayouts(tt.players)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatePayouts(t *testing.T) {
	tests := []struct {
		name    string
		spots   []PayoutSpot
		players int
		wantErr error
	}{
		{
			name:    "valid",
			spots:   []PayoutSpot{{Place: 1, Kind: PayoutRemainder}},
			players: 4,
			wantErr: nil,
		},
		{
			name: "too many places",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 1},
				{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
			},
			players: 2,
			wantErr: ErrTooManyPayoutPlaces,
		},
		{
			name: "duplicate place",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 1, Kind: PayoutFixed, BuyInsValue: 1},
			},
			players: 4,
			wantErr: ErrDuplicatePayoutPlace,
		},
		{
			name: "invalid fixed buy-in",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 0},
			},
			players: 4,
			wantErr: ErrInvalidBuyInsValue,
		},
		{
			name: "invalid payout kind",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutKind(99)},
			},
			players: 4,
			wantErr: ErrInvalidPayoutKind,
		},
		{
			name: "no remainder",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutFixed, BuyInsValue: 1},
			},
			players: 4,
			wantErr: ErrRemainderRequired,
		},
		{
			name: "two remainder",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutRemainder},
			},
			players: 4,
			wantErr: ErrRemainderRequired,
		},
		{
			name: "gap in places",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
			},
			players: 4,
			wantErr: ErrGapInPayoutPlaces,
		},
		{
			name: "fixed payouts exceed pot",
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 3},
			},
			players: 3,
			wantErr: ErrRemainderNotPositive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePayouts(tt.spots, tt.players)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
