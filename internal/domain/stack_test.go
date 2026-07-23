package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateStackPlan(t *testing.T) {
	tests := []struct {
		name    string
		players int
		chips   []ChipDenomination
		style   TournamentStyle
		want    StackPlan
		wantErr error
	}{
		{
			name:    "standard 4 players",
			players: 4,
			chips: []ChipDenomination{
				{Value: 5, Count: 40}, {Value: 25, Count: 40},
				{Value: 100, Count: 40}, {Value: 500, Count: 40},
			},
			style: StyleStandard,
			want: StackPlan{Distribution: []ChipDenomination{
				{Value: 5, Count: 10}, {Value: 25, Count: 10},
				{Value: 100, Count: 7}, {Value: 500, Count: 7},
			}},
			wantErr: nil,
		},
		{
			name:    "unknown style falls back to standard",
			players: 4,
			chips: []ChipDenomination{
				{Value: 5, Count: 40}, {Value: 25, Count: 40},
				{Value: 100, Count: 40}, {Value: 500, Count: 40},
			},
			style: TournamentStyle(999),
			want: StackPlan{Distribution: []ChipDenomination{
				{Value: 5, Count: 10}, {Value: 25, Count: 10},
				{Value: 100, Count: 7}, {Value: 500, Count: 7},
			}},
			wantErr: nil,
		},
		{
			name:    "not enough players",
			players: 1,
			chips:   []ChipDenomination{{Value: 25, Count: 40}},
			style:   StyleStandard,
			want:    StackPlan{},
			wantErr: ErrNotEnoughPlayers,
		},
		{
			name:    "incorrect chipset",
			players: 4,
			chips:   []ChipDenomination{{Value: 0, Count: 40}},
			style:   StyleStandard,
			want:    StackPlan{},
			wantErr: ErrIncorrectChipSet,
		},
		{
			name:    "duplicate chip denomination",
			players: 4,
			chips: []ChipDenomination{
				{Value: 25, Count: 40},
				{Value: 25, Count: 40},
			},
			style:   StyleStandard,
			want:    StackPlan{},
			wantErr: ErrDuplicateChipDenomination,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateStackPlan(tt.players, tt.chips, tt.style)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want.Distribution, got.Distribution)
		})
	}
}

func TestGenerateStackPlanDoesNotMutateChips(t *testing.T) {
	chips := []ChipDenomination{
		{Value: 500, Count: 40}, {Value: 5, Count: 40},
		{Value: 100, Count: 40}, {Value: 25, Count: 40},
	}
	want := []ChipDenomination{
		{Value: 500, Count: 40}, {Value: 5, Count: 40},
		{Value: 100, Count: 40}, {Value: 25, Count: 40},
	}

	_, err := GenerateStackPlan(4, chips, StyleStandard)

	assert.NoError(t, err)
	assert.Equal(t, want, chips)
}
