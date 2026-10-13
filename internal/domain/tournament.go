package domain

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SettlementMode string

const (
	SettlementBank SettlementMode = "bank"
	SettlementP2P  SettlementMode = "p2p"
)

type Rebuy struct {
	ID   uuid.UUID
	Paid bool
}

type TournamentPlayer struct {
	ID        uuid.UUID
	Name      string
	IsDeleted bool
	BuyInPaid bool
	Rebuys    []Rebuy
}

type Tournament struct {
	ID             uuid.UUID
	Name           string
	Date           time.Time
	Players        []TournamentPlayer
	BuyInAmount    int64
	SettlementMode SettlementMode
	ChipSet        []ChipDenomination
	Style          TournamentStyle
	Duration       time.Duration
	LevelDuration  time.Duration
	StartingStack  StackPlan
	BlindStructure []BlindLevel
	Results        []Result
}

func (t *Tournament) Pot() int64 {
	var entries int64
	for _, player := range t.Players {
		entries += 1 + int64(len(player.Rebuys))
	}
	return entries * t.BuyInAmount
}

func (t *Tournament) HasResults() bool { return len(t.Results) > 0 }

// NewTournament validates and copies a complete saved card.
func NewTournament(input Tournament) (*Tournament, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrEmptyName
	}
	if len(input.Players) < 2 {
		return nil, ErrNotEnoughPlayers
	}
	if input.BuyInAmount < 0 {
		return nil, ErrIncorrectBuyInAmount
	}
	if err := validateChipSet(input.ChipSet); err != nil {
		return nil, err
	}
	if input.Duration <= 0 || input.LevelDuration <= 0 {
		return nil, ErrIncorrectDuration
	}
	if input.Style < StyleStandard || input.Style > StyleDeep {
		return nil, ErrInvalidStyle
	}
	if input.SettlementMode != SettlementBank && input.SettlementMode != SettlementP2P {
		return nil, ErrInvalidSettlementMode
	}
	ids := make(map[uuid.UUID]bool, len(input.Players))
	rebuys := make(map[uuid.UUID]bool)
	var entries int64
	for _, player := range input.Players {
		name := strings.TrimSpace(player.Name)
		if name == "" {
			return nil, ErrInvalidPlayerName
		}
		if player.ID == uuid.Nil {
			return nil, ErrInvalidPlayerID
		}
		if ids[player.ID] {
			return nil, ErrDuplicatePlayer
		}
		ids[player.ID] = true
		if int64(len(player.Rebuys)) >= math.MaxInt64-entries {
			return nil, ErrPotOverflow
		}
		entries += 1 + int64(len(player.Rebuys))
		for _, rebuy := range player.Rebuys {
			if rebuy.ID == uuid.Nil {
				return nil, ErrInvalidRebuyID
			}
			if rebuys[rebuy.ID] {
				return nil, ErrDuplicateRebuy
			}
			rebuys[rebuy.ID] = true
		}
	}
	if input.BuyInAmount > math.MaxInt64/entries {
		return nil, ErrPotOverflow
	}
	if err := input.validateResults(ids); err != nil {
		return nil, err
	}
	output := input
	output.Players = slices.Clone(input.Players)
	for i := range output.Players {
		output.Players[i].Rebuys = slices.Clone(input.Players[i].Rebuys)
	}
	output.ChipSet = slices.Clone(input.ChipSet)
	output.StartingStack.Distribution = slices.Clone(input.StartingStack.Distribution)
	output.BlindStructure = slices.Clone(input.BlindStructure)
	output.Results = slices.Clone(input.Results)
	for i, result := range input.Results {
		if result.Place != nil {
			place := *result.Place
			output.Results[i].Place = &place
		}
	}
	return &output, nil
}
