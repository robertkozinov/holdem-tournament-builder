package postgres

import (
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
)

type tournamentPlayerProjection struct {
	playerID     uuid.UUID
	contribution int64
	prize        int64
	included     bool
}

func tournamentPlayerProjections(card *domain.Tournament) []tournamentPlayerProjection {
	prizes := make(map[uuid.UUID]int64, len(card.Results))
	for _, result := range card.Results {
		prizes[result.PlayerID] = result.Prize
	}

	included := card.HasResults()
	projections := make([]tournamentPlayerProjection, len(card.Players))
	for i, player := range card.Players {
		projections[i] = tournamentPlayerProjection{
			playerID:     player.ID,
			contribution: (1 + int64(len(player.Rebuys))) * card.BuyInAmount,
			prize:        prizes[player.ID],
			included:     included,
		}
	}
	return projections
}
