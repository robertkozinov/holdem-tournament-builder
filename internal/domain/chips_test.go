package domain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateChipSet_PreventsArithmeticOverflow(t *testing.T) {
	for _, chips := range [][]ChipDenomination{
		{{Value: math.MaxInt64, Count: 2}},
		{{Value: 1, Count: math.MaxInt}},
		{{Value: math.MaxInt64 / 100, Count: 1}, {Value: 1, Count: 1}},
	} {
		require.Error(t, validateChipSet(chips))
	}
}

func TestValidateChipSet_RejectsDuplicateValues(t *testing.T) {
	manyChips := make([]ChipDenomination, 800)
	for i := range manyChips {
		manyChips[i] = ChipDenomination{Value: 25000000000000, Count: 4}
	}
	tests := []struct {
		name  string
		chips []ChipDenomination
	}{
		{"two identical rows", []ChipDenomination{{Value: 500, Count: 40}, {Value: 500, Count: 40}}},
		{"non-adjacent values with different counts", []ChipDenomination{{Value: 500, Count: 40}, {Value: 25, Count: 80}, {Value: 500, Count: 20}}},
		{"many rows below inventory overflow limit", manyChips},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChipSet(tt.chips)
			require.ErrorIs(t, err, ErrDuplicateChipValue)
			require.EqualError(t, err, "chip denomination values must not be duplicated")
		})
	}
}

func TestValidateChipSet_PreservesExistingValidation(t *testing.T) {
	tests := []struct {
		name    string
		chips   []ChipDenomination
		wantErr error
	}{
		{"empty", nil, ErrEmptyChipSet},
		{"zero value", []ChipDenomination{{Value: 0, Count: 40}}, ErrIncorrectChipSet},
		{"negative value", []ChipDenomination{{Value: -500, Count: 40}}, ErrIncorrectChipSet},
		{"zero count", []ChipDenomination{{Value: 500, Count: 0}}, ErrIncorrectChipSet},
		{"negative count", []ChipDenomination{{Value: 500, Count: -40}}, ErrIncorrectChipSet},
		{"invalid duplicate row", []ChipDenomination{{Value: 500, Count: 40}, {Value: 500, Count: 0}}, ErrIncorrectChipSet},
		{"distinct values with equal counts", []ChipDenomination{{Value: 25, Count: 40}, {Value: 500, Count: 40}}, nil},
		{"inventory limit", []ChipDenomination{{Value: math.MaxInt64 / 100, Count: 1}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChipSet(tt.chips)
			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
