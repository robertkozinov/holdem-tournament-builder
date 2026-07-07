package domain

import (
	"slices"
	"time"
)

func (t *Tournament) AddRebuy(playerName string) error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	if !slices.Contains(t.Players, playerName) {
		return ErrPlayerNotFound
	}

	if !t.RebuyRules.Allowed || t.CurrentLevel > t.RebuyRules.MaxLevel {
		return ErrRebuyNotAllowed
	}
	t.Contributions[playerName] += t.BuyInAmount
	return nil
}

func (t *Tournament) LevelUp(now time.Time) error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	if t.CurrentLevel+1 < len(t.BlindStructure) {
		t.CurrentLevel++

		t.LevelStartedAt = now
		return nil
	}

	return ErrMaxBlindLevel
}

func (t *Tournament) CurrentBlindLevel() (BlindLevel, error) {
	if t.CurrentLevel < 0 || t.CurrentLevel >= len(t.BlindStructure) {
		return BlindLevel{}, ErrMaxBlindLevel
	}

	return t.BlindStructure[t.CurrentLevel], nil
}

func (t *Tournament) NextBlindLevel() (BlindLevel, bool) {
	nextLevel := t.CurrentLevel + 1
	if nextLevel < 0 || nextLevel >= len(t.BlindStructure) {
		return BlindLevel{}, false
	}

	return t.BlindStructure[nextLevel], true
}

func (t *Tournament) TimeLeftInLevel(now time.Time) (time.Duration, error) {
	level, err := t.CurrentBlindLevel()
	if err != nil {
		return 0, err
	}
	if level.Duration <= 0 {
		return 0, ErrIncorrectDuration
	}

	var elapsed time.Duration
	switch t.Status {
	case StatusCreated:
		return level.Duration, nil
	case StatusRunning:
		elapsed = now.Sub(t.LevelStartedAt)
	case StatusPaused:
		elapsed = t.PausedAt.Sub(t.LevelStartedAt)
	default:
		return 0, ErrIncorrectStatus
	}

	if elapsed <= 0 {
		return level.Duration, nil
	}
	if elapsed >= level.Duration {
		return 0, nil
	}

	return level.Duration - elapsed, nil
}

func (t *Tournament) AdvanceLevelIfNeeded(now time.Time) error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	if _, err := t.CurrentBlindLevel(); err != nil {
		return err
	}

	for t.CurrentLevel+1 < len(t.BlindStructure) {
		level, err := t.CurrentBlindLevel()
		if err != nil {
			return err
		}
		if level.Duration <= 0 {
			return ErrIncorrectDuration
		}
		if now.Sub(t.LevelStartedAt) < level.Duration {
			return nil
		}

		t.LevelStartedAt = t.LevelStartedAt.Add(level.Duration)
		t.CurrentLevel++
	}

	return nil
}

func (t *Tournament) Start(now time.Time) error {
	if t.Status != StatusCreated {
		return ErrIncorrectStatus
	}
	t.Status = StatusRunning
	t.LevelStartedAt = now
	return nil

}

func (t *Tournament) Pause(now time.Time) error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	t.Status = StatusPaused
	t.PausedAt = now
	return nil
}

func (t *Tournament) Resume(now time.Time) error {
	if t.Status != StatusPaused {
		return ErrIncorrectStatus
	}
	t.Status = StatusRunning
	t.LevelStartedAt = t.LevelStartedAt.Add(now.Sub(t.PausedAt))
	return nil
}

func (t *Tournament) Finish() error {
	if t.Status != StatusRunning {
		return ErrIncorrectStatus
	}
	if len(t.Players) != 1 {
		return ErrCantFinish
	}

	var prize int64

	payouts := t.CalculatePayouts()
	for _, v := range payouts {
		if v.Place == 1 {
			prize = v.Value
		}
	}

	t.Results = append(t.Results, Result{
		Name:  t.Players[0],
		Place: 1,
		Prize: prize,
	})

	t.Status = StatusFinished

	return nil

}
