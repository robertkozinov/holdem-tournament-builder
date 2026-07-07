package domain

import (
	"sort"
)

type StackPlan struct {
	Distribution []ChipDenomination
}

func (sp StackPlan) Total() int64 {
	var sum int64
	for _, c := range sp.Distribution {
		sum += c.Value * int64(c.Count)
	}
	return sum
}

type TournamentStyle int

const (
	StyleStandard TournamentStyle = iota
	StyleTurbo
	StyleDeep
)

var styleDepth = map[TournamentStyle]int64{
	StyleTurbo:    100,
	StyleStandard: 200,
	StyleDeep:     300,
}

const reservePercent = 30

func GenerateStackPlan(players int, chips []ChipDenomination, style TournamentStyle) (StackPlan, error) {
	if players < 2 {
		return StackPlan{}, ErrNotEnoughPlayers
	}
	if err := validateChipSet(chips); err != nil {
		return StackPlan{}, err
	}

	sorted := make([]ChipDenomination, len(chips))
	copy(sorted, chips)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value < sorted[j].Value
	})

	var totalValue int64
	for _, c := range sorted {
		totalValue += c.Value * int64(c.Count)
	}
	depth, ok := styleDepth[style]
	if !ok {
		depth = styleDepth[StyleStandard]
	}
	roughStack := totalValue * (100 - reservePercent) / 100 / int64(players)
	startBB := StartingBlind(roughStack, depth)
	startSB := startBB / 2

	minUseful := sorted[0].Value
	for _, c := range sorted {
		if startBB%c.Value == 0 && startSB%c.Value == 0 {
			minUseful = c.Value
		}
	}
	used := make([]ChipDenomination, 0, len(sorted))
	for _, c := range sorted {
		if c.Value >= minUseful {
			used = append(used, c)
		}
	}

	bigThreshold := startBB * 2
	dist := make(map[int64]int)
	var bigTotal int64
	for _, c := range used {
		if c.Value < bigThreshold {
			if per := c.Count / players; per > 0 {
				dist[c.Value] = per
			}
		} else {
			bigTotal += c.Value * int64(c.Count)
		}
	}

	remaining := bigTotal * (100 - reservePercent) / 100 / int64(players)
	for i := len(used) - 1; i >= 0; i-- {
		c := used[i]
		if c.Value < bigThreshold {
			continue
		}
		capPer := int64(c.Count) * (100 - reservePercent) / 100 / int64(players)
		give := remaining / c.Value
		if give > capPer {
			give = capPer
		}
		if give > 0 {
			dist[c.Value] += int(give)
			remaining -= give * c.Value
		}
	}

	distribution := make([]ChipDenomination, 0, len(dist))
	for _, c := range used {
		if n, ok := dist[c.Value]; ok {
			distribution = append(distribution, ChipDenomination{Value: c.Value, Count: n})
		}
	}

	plan := StackPlan{Distribution: distribution}

	if len(distribution) == 0 || plan.Total() == 0 {
		return StackPlan{}, ErrSmallStack
	}

	return plan, nil
}
