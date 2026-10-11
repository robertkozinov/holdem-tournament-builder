package domain

import "github.com/google/uuid"

type Result struct {
	PlayerID uuid.UUID
	// A nil place records a manual split without assigning a ranking.
	Place *int
	Prize int64
}

func (t *Tournament) validateResults(players map[uuid.UUID]bool) error {
	if !t.HasResults() {
		return nil
	}
	seenPlayers := make(map[uuid.UUID]bool, len(t.Results))
	seenPlaces := make(map[int]bool)
	var sum int64
	pot := t.Pot()
	for _, result := range t.Results {
		if !players[result.PlayerID] {
			return ErrPlayerNotFound
		}
		if seenPlayers[result.PlayerID] {
			return ErrDuplicateResult
		}
		seenPlayers[result.PlayerID] = true
		if result.Place != nil {
			place := *result.Place
			if place < 1 || place > len(t.Players) {
				return ErrInvalidResultPlace
			}
			if seenPlaces[place] {
				return ErrDuplicatePayoutPlace
			}
			seenPlaces[place] = true
		}
		if result.Prize <= 0 {
			return ErrInvalidPrize
		}
		if result.Prize > pot-sum {
			return ErrPrizeSumMismatch
		}
		sum += result.Prize
	}
	if sum != pot {
		return ErrPrizeSumMismatch
	}
	return nil
}
