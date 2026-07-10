package domain

import (
	"time"

	"github.com/google/uuid"
)

type Tournament struct {
	ID uuid.UUID

	//custom
	Name        string
	Date        time.Time
	Players     []string
	BuyInAmount int64
	ChipSet     []ChipDenomination
	RebuyRules  RebuyRules
	Duration    time.Duration

	//calculating
	StartingStack  StackPlan
	BlindStructure []BlindLevel
	PayoutSpots    []PayoutSpot

	//runtime
	Contributions  map[string]int64
	CurrentLevel   int
	LevelStartedAt time.Time
	PausedAt       time.Time
	Status         Status
	Results        []Result
}

type Status int

const (
	StatusCreated Status = iota
	StatusRunning
	StatusPaused
	StatusFinished
)

type RebuyRules struct {
	Allowed  bool
	MaxLevel int
}

func (t *Tournament) Pot() int64 {
	var pot int64
	for _, amount := range t.Contributions {
		pot += amount
	}
	return pot
}

func initContributions(players []string, buyIn int64) map[string]int64 {
	c := make(map[string]int64, len(players))
	for _, p := range players {
		c[p] = buyIn
	}
	return c
}

func hasDuplicates(players []string) bool {
	seen := make(map[string]bool, len(players))
	for _, p := range players {
		if seen[p] {
			return true
		}
		seen[p] = true
	}
	return false
}

func NewTournament(name string,
	date time.Time,
	players []string,
	buyInAmount int64,
	chips []ChipDenomination,
	rebuyRules RebuyRules,
	duration time.Duration,
	stack StackPlan,
	blinds []BlindLevel,
	payouts []PayoutSpot,

) (*Tournament, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	if len(players) < 2 {
		return nil, ErrNotEnoughPlayers
	}
	if hasDuplicates(players) {
		return nil, ErrDuplicatePlayer
	}
	if buyInAmount < 0 {
		return nil, ErrIncorrectBuyInAmount
	}
	if err := validateChipSet(chips); err != nil {
		return nil, err
	}
	if duration <= 0 {
		return nil, ErrIncorrectDuration
	}
	if rebuyRules.MaxLevel < 0 {
		return nil, ErrIncorrectRebuyRules
	}

	return &Tournament{
		Name:           name,
		Date:           date,
		Players:        players,
		BuyInAmount:    buyInAmount,
		ChipSet:        chips,
		RebuyRules:     rebuyRules,
		Duration:       duration,
		StartingStack:  stack,
		BlindStructure: blinds,
		Contributions:  initContributions(players, buyInAmount),
		PayoutSpots:    payouts,
		Status:         StatusCreated,
	}, nil
}
