package http

import (
	"holdem-tournament-builder/internal/domain"
	"time"
)

type TournamentResponse struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Date              time.Time            `json:"date"`
	Players           []string             `json:"players"`
	BuyInAmount       int64                `json:"buy_in_amount"`
	ChipSet           []ChipResponse       `json:"chip_set"`
	RebuyRules        RebuyRulesResponse   `json:"rebuy_rules"`
	DurationMinutes   int                  `json:"duration_minutes"`
	StartingStack     StackPlanResponse    `json:"starting_stack"`
	BlindStructure    []BlindLevelResponse `json:"blind_structure"`
	PayoutSpots       []PayoutSpotResponse `json:"payout_spots"`
	Contributions     map[string]int64     `json:"contributions"`
	Pot               int64                `json:"pot"`
	CurrentLevel      int                  `json:"current_level"`
	CurrentBlindLevel *BlindLevelResponse  `json:"current_blind_level,omitempty"`
	NextBlindLevel    *BlindLevelResponse  `json:"next_blind_level,omitempty"`
	LevelStartedAt    *time.Time           `json:"level_started_at,omitempty"`
	PausedAt          *time.Time           `json:"paused_at,omitempty"`
	Status            string               `json:"status"`
	Results           []ResultResponse     `json:"results"`
}

type ChipResponse struct {
	Value int64 `json:"value"`
	Count int   `json:"count"`
}

type RebuyRulesResponse struct {
	Allowed  bool `json:"allowed"`
	MaxLevel int  `json:"max_level"`
}

type StackPlanResponse struct {
	Distribution []ChipResponse `json:"distribution"`
	Total        int64          `json:"total"`
}

type BlindLevelResponse struct {
	SmallBlind      int64 `json:"small_blind"`
	BigBlind        int64 `json:"big_blind"`
	Ante            int64 `json:"ante"`
	DurationMinutes int   `json:"duration_minutes"`
}

type PayoutSpotResponse struct {
	Place       int    `json:"place"`
	Kind        string `json:"kind"`
	BuyInsValue int    `json:"buy_ins_value"`
}

type ResultResponse struct {
	Name  string `json:"name"`
	Place int    `json:"place"`
	Prize int64  `json:"prize"`
}

func newTournamentResponse(t *domain.Tournament) TournamentResponse {
	var currentBlind *BlindLevelResponse
	if level, err := t.CurrentBlindLevel(); err == nil {
		resp := newBlindLevelResponse(level)
		currentBlind = &resp
	}

	var nextBlind *BlindLevelResponse
	if level, ok := t.NextBlindLevel(); ok {
		resp := newBlindLevelResponse(level)
		nextBlind = &resp
	}

	return TournamentResponse{
		ID:                t.ID.String(),
		Name:              t.Name,
		Date:              t.Date,
		Players:           t.Players,
		BuyInAmount:       t.BuyInAmount,
		ChipSet:           newChipResponses(t.ChipSet),
		RebuyRules:        newRebuyRulesResponse(t.RebuyRules),
		DurationMinutes:   int(t.Duration / time.Minute),
		StartingStack:     newStackPlanResponse(t.StartingStack),
		BlindStructure:    newBlindLevelResponses(t.BlindStructure),
		PayoutSpots:       newPayoutSpotResponses(t.PayoutSpots),
		Contributions:     t.Contributions,
		Pot:               t.Pot(),
		CurrentLevel:      t.CurrentLevel,
		CurrentBlindLevel: currentBlind,
		NextBlindLevel:    nextBlind,
		LevelStartedAt:    timePtr(t.LevelStartedAt),
		PausedAt:          timePtr(t.PausedAt),
		Status:            tournamentStatusToString(t.Status),
		Results:           newResultResponses(t.Results),
	}
}

func newChipResponses(chips []domain.ChipDenomination) []ChipResponse {
	out := make([]ChipResponse, 0, len(chips))
	for _, chip := range chips {
		out = append(out, ChipResponse{
			Value: chip.Value,
			Count: chip.Count,
		})
	}
	return out
}

func newRebuyRulesResponse(rules domain.RebuyRules) RebuyRulesResponse {
	return RebuyRulesResponse{
		Allowed:  rules.Allowed,
		MaxLevel: rules.MaxLevel,
	}
}

func newStackPlanResponse(stack domain.StackPlan) StackPlanResponse {
	return StackPlanResponse{
		Distribution: newChipResponses(stack.Distribution),
		Total:        stack.Total(),
	}
}

func newBlindLevelResponse(level domain.BlindLevel) BlindLevelResponse {
	return BlindLevelResponse{
		SmallBlind:      level.SmallBlind,
		BigBlind:        level.BigBlind,
		Ante:            level.Ante,
		DurationMinutes: int(level.Duration / time.Minute),
	}
}

func newBlindLevelResponses(levels []domain.BlindLevel) []BlindLevelResponse {
	out := make([]BlindLevelResponse, 0, len(levels))
	for _, level := range levels {
		out = append(out, newBlindLevelResponse(level))
	}
	return out
}

func newPayoutSpotResponses(spots []domain.PayoutSpot) []PayoutSpotResponse {
	out := make([]PayoutSpotResponse, 0, len(spots))
	for _, spot := range spots {
		out = append(out, PayoutSpotResponse{
			Place:       spot.Place,
			Kind:        payoutKindToString(spot.Kind),
			BuyInsValue: spot.BuyInsValue,
		})
	}
	return out
}

func payoutKindToString(kind domain.PayoutKind) string {
	switch kind {
	case domain.PayoutFixed:
		return "fixed"
	case domain.PayoutRemainder:
		return "remainder"
	default:
		return "unknown"
	}
}

func newResultResponses(results []domain.Result) []ResultResponse {
	out := make([]ResultResponse, 0, len(results))
	for _, result := range results {
		out = append(out, ResultResponse{
			Name:  result.Name,
			Place: result.Place,
			Prize: result.Prize,
		})
	}
	return out
}

func tournamentStatusToString(status domain.Status) string {
	switch status {
	case domain.StatusCreated:
		return "created"
	case domain.StatusRunning:
		return "running"
	case domain.StatusPaused:
		return "paused"
	case domain.StatusFinished:
		return "finished"
	default:
		return "unknown"
	}
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
