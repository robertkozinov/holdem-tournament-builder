package http

import (
	"fmt"
	"github.com/google/uuid"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"math"
	"time"
)

type TournamentSettingsRequest struct {
	Name                 string          `json:"name"`
	Date                 time.Time       `json:"date"`
	Players              []PlayerRequest `json:"players"`
	SettlementMode       string          `json:"settlement_mode"`
	ChipSet              []ChipRequest   `json:"chip_set"`
	Style                string          `json:"style"`
	DurationMinutes      int             `json:"duration_minutes"`
	LevelDurationMinutes int             `json:"level_duration_minutes"`
}
type CreateTournamentRequest struct {
	TournamentSettingsRequest
	BuyInAmount int64 `json:"buy_in_amount"`
}
type SaveTournamentRequest struct {
	TournamentSettingsRequest
	Results []ResultRequest `json:"results"`
}
type PlayerRequest struct {
	ID        uuid.UUID      `json:"id,omitempty"`
	Name      string         `json:"name"`
	BuyInPaid bool           `json:"buy_in_paid"`
	Rebuys    []RebuyRequest `json:"rebuys"`
}
type RebuyRequest struct {
	ID   uuid.UUID `json:"id,omitempty"`
	Paid bool      `json:"paid"`
}
type ResultRequest struct {
	PlayerID uuid.UUID `json:"player_id"`
	Place    *int      `json:"place"`
	IsSplit  bool      `json:"is_split"`
	Prize    int64     `json:"prize"`
}
type ChipRequest struct {
	Value int64 `json:"value"`
	Count int   `json:"count"`
}
type CreateTournamentResponse struct {
	ID string `json:"id"`
}

func (r TournamentSettingsRequest) toInput() (service.SaveTournamentInput, error) {
	style, err := parseTournamentStyle(r.Style)
	if err != nil {
		return service.SaveTournamentInput{}, err
	}
	mode := domain.SettlementMode(r.SettlementMode)
	if mode != domain.SettlementBank && mode != domain.SettlementP2P {
		return service.SaveTournamentInput{}, domain.ErrInvalidSettlementMode
	}
	duration, err := minutesToDuration(r.DurationMinutes)
	if err != nil {
		return service.SaveTournamentInput{}, err
	}
	levelDuration, err := minutesToDuration(r.LevelDurationMinutes)
	if err != nil {
		return service.SaveTournamentInput{}, err
	}
	chips := make([]domain.ChipDenomination, 0, len(r.ChipSet))
	for _, chip := range r.ChipSet {
		chips = append(chips, domain.ChipDenomination{Value: chip.Value, Count: chip.Count})
	}
	players := make([]domain.TournamentPlayer, 0, len(r.Players))
	for _, player := range r.Players {
		rebuys := make([]domain.Rebuy, 0, len(player.Rebuys))
		for _, rebuy := range player.Rebuys {
			rebuys = append(rebuys, domain.Rebuy{ID: rebuy.ID, Paid: rebuy.Paid})
		}
		players = append(players, domain.TournamentPlayer{ID: player.ID, Name: player.Name, BuyInPaid: player.BuyInPaid, Rebuys: rebuys})
	}
	return service.SaveTournamentInput{
		Name: r.Name, Date: r.Date, Players: players, SettlementMode: mode, ChipSet: chips, Style: style, Duration: duration, LevelDuration: levelDuration,
	}, nil
}

func (r CreateTournamentRequest) ToServiceInput() (service.CreateTournamentInput, error) {
	in, err := r.TournamentSettingsRequest.toInput()
	if err != nil {
		return service.CreateTournamentInput{}, err
	}
	return service.CreateTournamentInput{
		Name: in.Name, Date: in.Date, Players: in.Players, BuyInAmount: r.BuyInAmount, SettlementMode: in.SettlementMode, ChipSet: in.ChipSet,
		Style: in.Style, Duration: in.Duration, LevelDuration: in.LevelDuration,
	}, nil
}
func (r SaveTournamentRequest) ToServiceInput() (service.SaveTournamentInput, error) {
	in, err := r.TournamentSettingsRequest.toInput()
	if err != nil {
		return service.SaveTournamentInput{}, err
	}
	in.Results = make([]domain.Result, 0, len(r.Results))
	for _, result := range r.Results {
		if result.IsSplit && result.Place != nil {
			return service.SaveTournamentInput{}, fmt.Errorf("split prize cannot have a place")
		}
		if !result.IsSplit && result.Place == nil {
			return service.SaveTournamentInput{}, fmt.Errorf("prize requires a place or is_split")
		}
		in.Results = append(in.Results, domain.Result{PlayerID: result.PlayerID, Place: result.Place, Prize: result.Prize})
	}
	return in, nil
}

func minutesToDuration(minutes int) (time.Duration, error) {
	if minutes <= 0 || int64(minutes) > math.MaxInt64/int64(time.Minute) {
		return 0, domain.ErrIncorrectDuration
	}
	return time.Duration(minutes) * time.Minute, nil
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
		return 0, domain.ErrInvalidStyle
	}
}
