package domain

import (
	"math"
	"time"
)

type BlindLevel struct {
	SmallBlind int64
	BigBlind   int64
	Ante       int64
	Duration   time.Duration
}

var blindGrid = []int64{10, 20, 30, 40, 50, 60, 80, 100, 150, 200, 300, 400, 500, 600, 800, 1000, 1500, 2000, 3000}

func niceBlind(raw int64) int64 {
	best := blindGrid[0]
	for _, v := range blindGrid {
		d, bd := v-raw, best-raw
		if d < 0 {
			d = -d
		}
		if bd < 0 {
			bd = -bd
		}
		if d < bd {
			best = v
		}
	}
	return best
}

func StartingBlind(stack int64, depth int64) int64 {
	return niceBlind(stack / depth)
}

func GenerateBlinds(stack StackPlan, style TournamentStyle, duration time.Duration, levelDuration time.Duration) ([]BlindLevel, error) {
	if duration <= 0 || levelDuration <= 0 {
		return nil, ErrIncorrectDuration
	}

	depth, ok := styleDepth[style]
	if !ok {
		depth = styleDepth[StyleStandard]
	}

	startBB := StartingBlind(stack.Total(), depth)

	finalBB := stack.Total() / 8
	if finalBB == 0 {
		return nil, ErrSmallStack
	}

	levelsAmount := int(duration / levelDuration)
	if levelsAmount < 2 {
		return nil, ErrNotEnoughLevels
	}

	growth := math.Pow(float64(finalBB)/float64(startBB), 1.0/float64(levelsAmount-1))

	blinds := make([]BlindLevel, 0)

	var prevBlind int64
	for i := range levelsAmount {
		var ante int64

		raw := float64(startBB) * math.Pow(growth, float64(i))
		bb := niceBlind(int64(raw))

		if bb <= prevBlind {
			for _, v := range blindGrid {
				if v > prevBlind {
					bb = v
					break
				}
			}
		}
		prevBlind = bb

		if i >= levelsAmount/2 {
			ante = bb
		}

		blinds = append(blinds, BlindLevel{
			SmallBlind: bb / 2,
			BigBlind:   bb,
			Ante:       ante,
			Duration:   levelDuration,
		})
	}

	return blinds, nil
}
