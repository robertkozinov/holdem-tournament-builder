package domain

import "math"

type ChipDenomination struct {
	Value int64
	Count int
}

func validateChipSet(chips []ChipDenomination) error {
	if len(chips) < 1 {
		return ErrEmptyChipSet
	}

	// Reserve headroom for percentage multiplication in the stack generator.
	const maxTotal = math.MaxInt64 / 100
	var total int64
	seenValues := make(map[int64]bool, len(chips))
	for _, c := range chips {
		if c.Value <= 0 || c.Count <= 0 {
			return ErrIncorrectChipSet
		}
		if seenValues[c.Value] {
			return ErrDuplicateChipValue
		}
		seenValues[c.Value] = true
		if c.Value > (maxTotal-total)/int64(c.Count) {
			return ErrChipSetOverflow
		}
		total += c.Value * int64(c.Count)
	}

	return nil
}
