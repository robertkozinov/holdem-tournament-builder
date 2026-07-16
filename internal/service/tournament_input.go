package service

import (
	"holdem-tournament-builder/internal/domain"
	"time"
)

type PayoutMode string

const (
	PayoutModeDefault PayoutMode = "default"
	PayoutModeCustom  PayoutMode = "custom"
)

type CreateTournamentInput struct {
	Name              string
	Date              time.Time
	Players           []string
	BuyInAmount       int64
	Chips             []domain.ChipDenomination
	Style             domain.TournamentStyle
	Duration          time.Duration
	LevelDuration     time.Duration
	RebuyAllowed      bool
	RebuyMaxLevel     int
	PayoutMode        PayoutMode
	PayoutFixedBuyIns []int
}
