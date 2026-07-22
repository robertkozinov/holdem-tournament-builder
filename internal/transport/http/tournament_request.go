package http

import (
	"fmt"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"time"
)

type CreateTournamentRequest struct {
	Name                 string        `json:"name"`
	Date                 time.Time     `json:"date"`
	Players              []string      `json:"players"`
	BuyInAmount          int64         `json:"buy_in_amount"`
	Chips                []ChipRequest `json:"chips"`
	Style                string        `json:"style"`
	DurationMinutes      int           `json:"duration_minutes"`
	LevelDurationMinutes int           `json:"level_duration_minutes"`
	Rebuy                RebuyRequest  `json:"rebuy"`
	Payout               PayoutRequest `json:"payout"`
}

type ChipRequest struct {
	Value int64 `json:"value"`
	Count int   `json:"count"`
}

type RebuyRequest struct {
	Allowed  bool `json:"allowed"`
	MaxLevel int  `json:"max_level"`
}

type PayoutRequest struct {
	PayoutMode  string `json:"payout_mode"`
	FixedBuyIns []int  `json:"fixed_buy_ins"`
}

type CreateTournamentResponse struct {
	ID string `json:"id"`
}

func (r CreateTournamentRequest) ToServiceInput() (service.CreateTournamentInput, error) {
	chips := make([]domain.ChipDenomination, 0, len(r.Chips))
	for _, chip := range r.Chips {
		chips = append(chips, domain.ChipDenomination{
			Value: chip.Value,
			Count: chip.Count,
		})
	}

	style, err := parseTournamentStyle(r.Style)
	if err != nil {
		return service.CreateTournamentInput{}, err
	}

	input := service.CreateTournamentInput{
		Name:              r.Name,
		Date:              r.Date,
		Players:           r.Players,
		BuyInAmount:       r.BuyInAmount,
		Chips:             chips,
		Style:             style,
		Duration:          time.Duration(r.DurationMinutes) * time.Minute,
		LevelDuration:     time.Duration(r.LevelDurationMinutes) * time.Minute,
		RebuyAllowed:      r.Rebuy.Allowed,
		RebuyMaxLevel:     r.Rebuy.MaxLevel,
		PayoutMode:        service.PayoutMode(r.Payout.PayoutMode),
		PayoutFixedBuyIns: r.Payout.FixedBuyIns,
	}

	return input, nil
}

func parseTournamentStyle(style string) (domain.TournamentStyle, error) {
	switch style {
	case "standard":
		return domain.StyleStandard, nil
	case "turbo":
		return domain.StyleTurbo, nil
	case "deep":
		return domain.StyleDeep, nil
	default:
		return 0, fmt.Errorf("invalid tournament style")
	}
}
