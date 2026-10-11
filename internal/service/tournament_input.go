package service

import (
	"holdem-tournament-builder/internal/domain"
	"time"
)

type CreateTournamentInput struct {
	Name           string
	Date           time.Time
	Players        []domain.TournamentPlayer
	BuyInAmount    int64
	SettlementMode domain.SettlementMode
	ChipSet        []domain.ChipDenomination
	Style          domain.TournamentStyle
	Duration       time.Duration
	LevelDuration  time.Duration
}

type SaveTournamentInput struct {
	Name           string
	Date           time.Time
	Players        []domain.TournamentPlayer
	SettlementMode domain.SettlementMode
	ChipSet        []domain.ChipDenomination
	Style          domain.TournamentStyle
	Duration       time.Duration
	LevelDuration  time.Duration
	Results        []domain.Result
}
