package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateBlinds(t *testing.T) {
	tests := []struct {
		name          string
		stack         StackPlan
		style         TournamentStyle
		duration      time.Duration
		levelDuration time.Duration
		want          []BlindLevel
		wantErr       error
	}{
		{
			name: "standard blinds",
			stack: StackPlan{Distribution: []ChipDenomination{
				{Value: 1000, Count: 8},
			}},
			style:         StyleStandard,
			duration:      time.Hour,
			levelDuration: 20 * time.Minute,
			want: []BlindLevel{
				{SmallBlind: 20, BigBlind: 40, Ante: 0, Duration: 20 * time.Minute},
				{SmallBlind: 100, BigBlind: 200, Ante: 200, Duration: 20 * time.Minute},
				{SmallBlind: 500, BigBlind: 1000, Ante: 1000, Duration: 20 * time.Minute},
			},
			wantErr: nil,
		},
		{
			name:          "too small stack",
			stack:         StackPlan{Distribution: []ChipDenomination{{Value: 0, Count: 0}}},
			style:         StyleStandard,
			duration:      2 * time.Hour,
			levelDuration: 20 * time.Minute,
			wantErr:       ErrSmallStack,
		},
		{
			name: "not enough levels",
			stack: StackPlan{Distribution: []ChipDenomination{
				{Value: 1000, Count: 8},
			}},
			style:         StyleStandard,
			duration:      20 * time.Minute,
			levelDuration: 20 * time.Minute,
			wantErr:       ErrNotEnoughLevels,
		},
		{
			name: "negative level duration",
			stack: StackPlan{Distribution: []ChipDenomination{
				{Value: 1000, Count: 8},
			}},
			style:         StyleStandard,
			duration:      time.Hour,
			levelDuration: -time.Minute,
			wantErr:       ErrIncorrectDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateBlinds(tt.stack, tt.style, tt.duration, tt.levelDuration)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateBlindsUsesStyleLevelDurationWhenNotProvided(t *testing.T) {
	tests := []struct {
		name         string
		style        TournamentStyle
		wantDuration time.Duration
		wantLevels   int
	}{
		{name: "turbo", style: StyleTurbo, wantDuration: 10 * time.Minute, wantLevels: 6},
		{name: "standard", style: StyleStandard, wantDuration: 20 * time.Minute, wantLevels: 3},
		{name: "deep", style: StyleDeep, wantDuration: 30 * time.Minute, wantLevels: 2},
	}

	stack := StackPlan{Distribution: []ChipDenomination{{Value: 1000, Count: 8}}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateBlinds(stack, tt.style, time.Hour, 0)

			assert.NoError(t, err)
			if assert.Len(t, got, tt.wantLevels) {
				for _, level := range got {
					assert.Equal(t, tt.wantDuration, level.Duration)
				}
			}
		})
	}
}

func TestGenerateBlindsUsesProvidedLevelDuration(t *testing.T) {
	stack := StackPlan{Distribution: []ChipDenomination{{Value: 1000, Count: 8}}}

	got, err := GenerateBlinds(stack, StyleTurbo, 45*time.Minute, 15*time.Minute)

	assert.NoError(t, err)
	if assert.Len(t, got, 3) {
		for _, level := range got {
			assert.Equal(t, 15*time.Minute, level.Duration)
		}
	}
}
