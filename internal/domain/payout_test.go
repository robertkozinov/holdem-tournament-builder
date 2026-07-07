package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculatePayouts(t *testing.T) {
	tests := []struct {
		name          string
		buyIn         int64
		spots         []PayoutSpot
		contributions map[string]int64
		want          []PayoutResult
	}{
		{
			name:          "winner takes all",
			buyIn:         1000,
			spots:         []PayoutSpot{{Place: 1, Kind: PayoutRemainder}},
			contributions: map[string]int64{"A": 1000, "B": 1000, "C": 1000, "D": 1000},
			want:          []PayoutResult{{Place: 1, Value: 4000}},
		},
		{
			name:  "fixed places and remainder",
			buyIn: 1000,
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 2},
				{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
			},
			contributions: map[string]int64{"A": 1000, "B": 1000, "C": 1000, "D": 1000, "E": 1000},
			want: []PayoutResult{
				{Place: 1, Value: 2000},
				{Place: 2, Value: 2000},
				{Place: 3, Value: 1000},
			},
		},
		{
			name:  "rebuys increase remainder",
			buyIn: 1000,
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 1},
			},
			contributions: map[string]int64{"A": 1000, "B": 1000, "C": 3000, "D": 1000},
			want: []PayoutResult{
				{Place: 1, Value: 5000},
				{Place: 2, Value: 1000},
			},
		},
		{
			name:  "remainder not first",
			buyIn: 1000,
			spots: []PayoutSpot{
				{Place: 1, Kind: PayoutFixed, BuyInsValue: 1},
				{Place: 2, Kind: PayoutRemainder},
			},
			contributions: map[string]int64{"A": 1000, "B": 1000, "C": 1000, "D": 1000},
			want: []PayoutResult{
				{Place: 1, Value: 1000},
				{Place: 2, Value: 3000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Tournament{
				BuyInAmount:   tt.buyIn,
				PayoutSpots:   tt.spots,
				Contributions: tt.contributions,
			}
			got := tr.CalculatePayouts()
			assert.Equal(t, tt.want, got)
		})
	}
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

func TestCustomPayouts(t *testing.T) {
	tests := []struct {
		name        string
		fixedBuyIns []int
		players     int
		want        []PayoutSpot
		wantErr     error
	}{
		{
			name:        "top 3",
			fixedBuyIns: []int{1, 1},
			players:     4,
			want: []PayoutSpot{
				{Place: 1, Kind: PayoutRemainder},
				{Place: 2, Kind: PayoutFixed, BuyInsValue: 1},
				{Place: 3, Kind: PayoutFixed, BuyInsValue: 1},
			},
			wantErr: nil,
		},
		{
			name:        "invalid fixed buy-in",
			fixedBuyIns: []int{0},
			players:     4,
			wantErr:     ErrInvalidBuyInsValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CustomPayouts(tt.fixedBuyIns, tt.players)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
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
