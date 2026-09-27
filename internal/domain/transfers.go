package domain

import "sort"

type Transfer struct {
	From   string
	To     string
	Amount int64
}

type balance struct {
	name   string
	amount int64
}

func (t *Tournament) balances() map[string]int64 {
	b := make(map[string]int64)
	for player, paid := range t.Contributions {
		b[player] = -paid
	}
	for _, r := range t.Results {
		b[r.Name] += r.Prize
	}
	return b
}

func minimizeTransfers(balances map[string]int64) []Transfer {
	var debtors, creditors []balance

	for name, b := range balances {
		switch {
		case b < 0:
			debtors = append(debtors, balance{name, -b})
		case b > 0:
			creditors = append(creditors, balance{name, b})
		}
	}

	sort.Slice(debtors, func(i, j int) bool { return debtors[i].name < debtors[j].name })
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].name < creditors[j].name })

	var transfers []Transfer
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		amount := min(debtors[i].amount, creditors[j].amount)

		transfers = append(transfers, Transfer{
			From:   debtors[i].name,
			To:     creditors[j].name,
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
	return minimizeTransfers(t.balances())
}
