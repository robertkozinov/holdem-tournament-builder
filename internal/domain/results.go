package domain

import (
	"slices"
)

type Result struct {
	Name  string
	Place int
	Prize int64
}

func (t *Tournament) Knockout(playerName string) error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	i := slices.Index(t.Players, playerName)
	if i < 0 {
		return ErrPlayerNotFound
	}
	if len(t.Players) <= 1 {
		return ErrCantKnockout
	}

	place := len(t.Players)
	var prize int64

	payouts := t.CalculatePayouts()
	for _, v := range payouts {
		if v.Place == place {
			prize = v.Value
		}
	}

	t.Results = append(t.Results, Result{
		Name:  playerName,
		Place: place,
		Prize: prize,
	})

	t.Players = slices.Delete(t.Players, i, i+1)

	return nil

}
