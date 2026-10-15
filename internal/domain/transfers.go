package domain

import (
	"sort"

	"github.com/google/uuid"
)

type Transfer struct {
	From   uuid.UUID
	To     uuid.UUID
	Amount int64
}

type balance struct {
	id     uuid.UUID
	amount int64
}

func (t *Tournament) balances() map[uuid.UUID]int64 {
	b := make(map[uuid.UUID]int64)
	for _, player := range t.Players {
		b[player.ID] = -(1 + int64(len(player.Rebuys))) * t.BuyInAmount
		for _, result := range t.Results {
			if result.PlayerID == player.ID {
				b[player.ID] += result.Prize
			}
		}
	}
	return b
}

func minimizeTransfers(balances map[uuid.UUID]int64) []Transfer {
	var debtors, creditors []balance

	for id, b := range balances {
		switch {
		case b < 0:
			debtors = append(debtors, balance{id, -b})
		case b > 0:
			creditors = append(creditors, balance{id, b})
		}
	}

	sort.Slice(debtors, func(i, j int) bool { return debtors[i].id.String() < debtors[j].id.String() })
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].id.String() < creditors[j].id.String() })

	var transfers []Transfer
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		amount := min(debtors[i].amount, creditors[j].amount)

		transfers = append(transfers, Transfer{
			From:   debtors[i].id,
			To:     creditors[j].id,
			Amount: amount,
		})

		debtors[i].amount -= amount
		creditors[j].amount -= amount

		if debtors[i].amount == 0 {
			i++
		}
		if creditors[j].amount == 0 {
			j++
		}
	}

	return transfers
}

func (t *Tournament) CalculateTransfers() []Transfer {
	if t.SettlementMode != SettlementP2P || !t.HasResults() {
		return nil
	}
	return minimizeTransfers(t.balances())
}
