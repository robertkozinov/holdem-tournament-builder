package domain

import "sort"

type PayoutKind int

const (
	PayoutFixed PayoutKind = iota
	PayoutRemainder
)

type PayoutSpot struct {
	Place       int
	Kind        PayoutKind
	BuyInsValue int
}

type PayoutResult struct {
	Place int
	Value int64
}

func (t *Tournament) CalculatePayouts() []PayoutResult {
	var res []PayoutResult

	buyInsCounter := 0

	for _, v := range t.PayoutSpots {
		if v.Kind == PayoutFixed {
			res = append(res, PayoutResult{
				Place: v.Place,
				Value: int64(v.BuyInsValue) * t.BuyInAmount,
			})
			buyInsCounter += v.BuyInsValue
		}
	}

	for _, v := range t.PayoutSpots {
		if v.Kind == PayoutRemainder {
			res = append(res, PayoutResult{Place: v.Place, Value: t.Pot() - int64(buyInsCounter)*t.BuyInAmount})
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Place < res[j].Place
	})

	return res
}

func buildPayouts(fixedBuyIns []int) []PayoutSpot {
	spots := []PayoutSpot{
		{Place: 1, Kind: PayoutRemainder},
	}
	for i, b := range fixedBuyIns {
		spots = append(spots, PayoutSpot{
			Place:       i + 2,
			Kind:        PayoutFixed,
			BuyInsValue: b,
		})
	}
	return spots
}

func validatePayouts(spots []PayoutSpot, players int) error {
	if len(spots) > players {
		return ErrTooManyPayoutPlaces
	}

	remainders := 0
	sumFixed := 0
	places := make(map[int]bool)

	for _, s := range spots {
		if places[s.Place] {
			return ErrDuplicatePayoutPlace
		}
		places[s.Place] = true

		switch s.Kind {
		case PayoutRemainder:
			remainders++
		case PayoutFixed:
			if s.BuyInsValue <= 0 {
				return ErrInvalidBuyInsValue
			}
			sumFixed += s.BuyInsValue
		default:
			return ErrInvalidPayoutKind
		}
	}

	if remainders != 1 {
		return ErrRemainderRequired
	}

	for p := 1; p <= len(spots); p++ {
		if !places[p] {
			return ErrGapInPayoutPlaces
		}
	}

	if sumFixed >= players {
		return ErrRemainderNotPositive
	}

	return nil
}

func CustomPayouts(fixedBuyIns []int, players int) ([]PayoutSpot, error) {
	spots := buildPayouts(fixedBuyIns)
	if err := validatePayouts(spots, players); err != nil {
		return nil, err
	}
	return spots, nil
}

func DefaultPayouts(players int) ([]PayoutSpot, error) {
	var fixed []int
	switch {
	case players <= 4:
		fixed = nil
	case players < 8:
		fixed = []int{1}
	default:
		fixed = []int{2, 1}
	}

	spots := buildPayouts(fixed)
	if err := validatePayouts(spots, players); err != nil {
		return nil, err
	}

	return spots, nil
}
