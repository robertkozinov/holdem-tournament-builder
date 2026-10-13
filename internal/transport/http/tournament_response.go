package http

import (
	"holdem-tournament-builder/internal/domain"
	"time"
)

type TournamentResponse struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Date                 time.Time            `json:"date"`
	Players              []PlayerResponse     `json:"players"`
	BuyInAmount          int64                `json:"buy_in_amount"`
	SettlementMode       string               `json:"settlement_mode"`
	ChipSet              []ChipResponse       `json:"chip_set"`
	Style                string               `json:"style"`
	DurationMinutes      int                  `json:"duration_minutes"`
	LevelDurationMinutes int                  `json:"level_duration_minutes"`
	StartingStack        StackPlanResponse    `json:"starting_stack"`
	BlindStructure       []BlindLevelResponse `json:"blind_structure"`
	SuggestedPayouts     []PayoutResponse     `json:"suggested_payouts"`
	Pot                  int64                `json:"pot"`
	HasResults           bool                 `json:"has_results"`
	Results              []ResultResponse     `json:"results"`
	Transfers            []TransferResponse   `json:"transfers"`
}
type PlayerResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	IsDeleted    bool            `json:"is_deleted"`
	BuyInPaid    bool            `json:"buy_in_paid"`
	Rebuys       []RebuyResponse `json:"rebuys"`
	Contribution int64           `json:"contribution"`
}
type RebuyResponse struct {
	ID   string `json:"id"`
	Paid bool   `json:"paid"`
}
type ChipResponse struct {
	Value int64 `json:"value"`
	Count int   `json:"count"`
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
type PayoutResponse struct {
	Place int   `json:"place"`
	Value int64 `json:"value"`
}
type ResultResponse struct {
	PlayerID  string `json:"player_id"`
	Name      string `json:"name"`
	IsDeleted bool   `json:"is_deleted"`
	Place     *int   `json:"place"`
	IsSplit   bool   `json:"is_split"`
	Prize     int64  `json:"prize"`
}
type TransferResponse struct {
	From          string `json:"from"`
	To            string `json:"to"`
	FromPlayerID  string `json:"from_player_id"`
	ToPlayerID    string `json:"to_player_id"`
	FromIsDeleted bool   `json:"from_is_deleted"`
	ToIsDeleted   bool   `json:"to_is_deleted"`
	Amount        int64  `json:"amount"`
}

func newTournamentResponse(t *domain.Tournament) (TournamentResponse, error) {
	payouts, err := t.CalculatePayouts()
	if err != nil {
		return TournamentResponse{}, err
	}
	players := make([]PlayerResponse, 0, len(t.Players))
	names := make(map[string]string, len(t.Players))
	deleted := make(map[string]bool, len(t.Players))
	for _, player := range t.Players {
		rebuys := make([]RebuyResponse, 0, len(player.Rebuys))
		for _, rebuy := range player.Rebuys {
			rebuys = append(rebuys, RebuyResponse{ID: rebuy.ID.String(), Paid: rebuy.Paid})
		}
		players = append(players, PlayerResponse{ID: player.ID.String(), Name: player.Name, IsDeleted: player.IsDeleted, BuyInPaid: player.BuyInPaid, Rebuys: rebuys, Contribution: (1 + int64(len(player.Rebuys))) * t.BuyInAmount})
		names[player.ID.String()] = player.Name
		deleted[player.ID.String()] = player.IsDeleted
	}
	suggested := make([]PayoutResponse, 0, len(payouts))
	for _, payout := range payouts {
		suggested = append(suggested, PayoutResponse{Place: payout.Place, Value: payout.Value})
	}
	results := make([]ResultResponse, 0, len(t.Results))
	for _, result := range t.Results {
		results = append(results, ResultResponse{
			PlayerID: result.PlayerID.String(), Name: names[result.PlayerID.String()], IsDeleted: deleted[result.PlayerID.String()], Place: result.Place, IsSplit: result.Place == nil, Prize: result.Prize,
		})
	}
	transfers := make([]TransferResponse, 0)
	for _, transfer := range t.CalculateTransfers() {
		transfers = append(transfers, TransferResponse{From: names[transfer.From.String()], To: names[transfer.To.String()], FromPlayerID: transfer.From.String(), ToPlayerID: transfer.To.String(), FromIsDeleted: deleted[transfer.From.String()], ToIsDeleted: deleted[transfer.To.String()], Amount: transfer.Amount})
	}
	return TournamentResponse{
		ID: t.ID.String(), Name: t.Name, Date: t.Date, Players: players, BuyInAmount: t.BuyInAmount, SettlementMode: string(t.SettlementMode),
		ChipSet: newChipResponses(t.ChipSet), Style: tournamentStyleToString(t.Style), DurationMinutes: int(t.Duration / time.Minute), LevelDurationMinutes: int(t.LevelDuration / time.Minute),
		StartingStack:  StackPlanResponse{Distribution: newChipResponses(t.StartingStack.Distribution), Total: t.StartingStack.Total()},
		BlindStructure: newBlindLevelResponses(t.BlindStructure), SuggestedPayouts: suggested, Pot: t.Pot(), HasResults: t.HasResults(), Results: results, Transfers: transfers,
	}, nil
}

func newChipResponses(chips []domain.ChipDenomination) []ChipResponse {
	out := make([]ChipResponse, 0, len(chips))
	for _, chip := range chips {
		out = append(out, ChipResponse{Value: chip.Value, Count: chip.Count})
	}
	return out
}
func newBlindLevelResponses(levels []domain.BlindLevel) []BlindLevelResponse {
	out := make([]BlindLevelResponse, 0, len(levels))
	for _, level := range levels {
		out = append(out, BlindLevelResponse{SmallBlind: level.SmallBlind, BigBlind: level.BigBlind, Ante: level.Ante, DurationMinutes: int(level.Duration / time.Minute)})
	}
	return out
}
func tournamentStyleToString(style domain.TournamentStyle) string {
	switch style {
	case domain.StyleStandard:
		return "standard"
	case domain.StyleTurbo:
		return "turbo"
	case domain.StyleDeep:
		return "deep"
	default:
		return "unknown"
	}
}
