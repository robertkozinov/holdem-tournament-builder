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
			name: "zero level duration",
			stack: StackPlan{Distribution: []ChipDenomination{
				{Value: 1000, Count: 8},
			}},
			style:         StyleStandard,
			duration:      time.Hour,
			levelDuration: 0,
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
