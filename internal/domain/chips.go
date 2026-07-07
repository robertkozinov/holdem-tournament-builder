package domain

type ChipDenomination struct {
	Value int64
	Count int
}

func validateChipSet(chips []ChipDenomination) error {
	if len(chips) < 1 {
		return ErrEmptyChipSet
	}

	for _, c := range chips {
		if c.Value <= 0 || c.Count <= 0 {
			return ErrIncorrectChipSet
		}
	}

	return nil
}
