package domain

type ChipDenomination struct {
	Value int64
	Count int
}

func validateChipSet(chips []ChipDenomination) error {
	if len(chips) < 1 {
		return ErrEmptyChipSet
	}

	values := make(map[int64]struct{}, len(chips))
	for _, c := range chips {
		if c.Value <= 0 || c.Count <= 0 {
			return ErrIncorrectChipSet
		}
		if _, exists := values[c.Value]; exists {
			return ErrDuplicateChipDenomination
		}
		values[c.Value] = struct{}{}
	}

	return nil
}
