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
	var total int64

	for name, b := range balances {
		total += b
		switch {
		case b < 0:
			debtors = append(debtors, balance{name, -b})
		case b > 0:
			creditors = append(creditors, balance{name, b})
		}
	}

	sort.Slice(debtors, func(i, j int) bool { return debtors[i].name < debtors[j].name })
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].name < creditors[j].name })

	if total != 0 {
		return nil
	}

	return findMinimumTransfers(debtors, creditors)
}

func findMinimumTransfers(debtors, creditors []balance) []Transfer {
	debtorIndex := -1
	for i := range debtors {
		if debtors[i].amount > 0 {
			debtorIndex = i
			break
		}
	}
	if debtorIndex < 0 {
		return nil
	}

	var best []Transfer
	for creditorIndex := range creditors {
		if creditors[creditorIndex].amount == 0 {
			continue
		}

		amount := min(debtors[debtorIndex].amount, creditors[creditorIndex].amount)
		debtors[debtorIndex].amount -= amount
		creditors[creditorIndex].amount -= amount

		remaining := findMinimumTransfers(debtors, creditors)
		candidate := make([]Transfer, 0, len(remaining)+1)
		candidate = append(candidate, Transfer{
			From:   debtors[debtorIndex].name,
			To:     creditors[creditorIndex].name,
			Amount: amount,
		})
		candidate = append(candidate, remaining...)

		debtors[debtorIndex].amount += amount
		creditors[creditorIndex].amount += amount

		if best == nil || len(candidate) < len(best) {
			best = candidate
		}
	}

	return best
}

func (t *Tournament) CalculateTransfers() []Transfer {
	return minimizeTransfers(t.balances())
}
