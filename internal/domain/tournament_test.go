package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTournament(t *testing.T) {
	tests := []struct {
		name       string
		tName      string
		players    []string
		buyIn      int64
		chips      []ChipDenomination
		duration   time.Duration
		rebuyRules RebuyRules
		wantErr    error
	}{
		{
			name:       "valid tournament",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    nil,
		},
		{
			name:       "empty name",
			tName:      "",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrEmptyName,
		},
		{
			name:       "1 player",
			tName:      "Friday Game",
			players:    []string{"A"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrNotEnoughPlayers,
		},
		{
			name:       "duplicate player",
			tName:      "Friday Game",
			players:    []string{"A", "A", "B", "C"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrDuplicatePlayer,
		},
		{
			name:       "negative buy-in",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      -1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrIncorrectBuyInAmount,
		},
		{
			name:       "empty chipset",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrEmptyChipSet,
		},
		{
			name:       "incorrect chip value",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 0, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrIncorrectChipSet,
		},
		{
			name:       "incorrect chip count",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 0}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrIncorrectChipSet,
		},
		{
			name:       "zero duration",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   0,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrIncorrectDuration,
		},
		{
			name:       "negative duration",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   -1 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: 3},
			wantErr:    ErrIncorrectDuration,
		},
		{
			name:       "negative rebuy max level",
			tName:      "Friday Game",
			players:    []string{"A", "B", "C", "D"},
			buyIn:      1000,
			chips:      []ChipDenomination{{Value: 25, Count: 50}, {Value: 100, Count: 50}},
			duration:   2 * time.Hour,
			rebuyRules: RebuyRules{Allowed: true, MaxLevel: -1},
			wantErr:    ErrIncorrectRebuyRules,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTournament(
				tt.tName, time.Now(), tt.players, tt.buyIn, tt.chips,
				tt.rebuyRules, tt.duration, StackPlan{}, nil, nil,
			)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.tName, got.Name)
				assert.Equal(t, tt.players, got.Players)
				assert.Equal(t, tt.buyIn, got.BuyInAmount)
				assert.Equal(t, tt.chips, got.ChipSet)
				assert.Equal(t, tt.rebuyRules, got.RebuyRules)
				assert.Equal(t, tt.duration, got.Duration)
				assert.Equal(t, StatusCreated, got.Status)
				assert.Equal(t, int64(len(tt.players))*tt.buyIn, got.Pot())
				for _, player := range tt.players {
					assert.Equal(t, tt.buyIn, got.Contributions[player])
				}
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
