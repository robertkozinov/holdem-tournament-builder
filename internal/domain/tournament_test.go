package domain

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tournamentFixture() Tournament {
	return Tournament{
		Name: "Friday Game", Date: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
		Players:     []TournamentPlayer{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}, {ID: uuid.New(), Name: "C"}, {ID: uuid.New(), Name: "D"}},
		BuyInAmount: 1000, SettlementMode: SettlementP2P,
		ChipSet: []ChipDenomination{{Value: 25, Count: 80}, {Value: 100, Count: 80}},
		Style:   StyleStandard, Duration: 2 * time.Hour, LevelDuration: 20 * time.Minute,
	}
}

func TestNewTournament(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*Tournament)
		wantErr error
	}{
		{"empty name", func(tr *Tournament) { tr.Name = " " }, ErrEmptyName},
		{"one player", func(tr *Tournament) { tr.Players = tr.Players[:1] }, ErrNotEnoughPlayers},
		{"duplicate name", func(tr *Tournament) { tr.Players[1].Name = tr.Players[0].Name }, ErrDuplicatePlayer},
		{"empty player name", func(tr *Tournament) { tr.Players[0].Name = " " }, ErrInvalidPlayerName},
		{"empty player id", func(tr *Tournament) { tr.Players[0].ID = uuid.Nil }, ErrInvalidPlayerID},
		{"duplicate player id", func(tr *Tournament) { tr.Players[1].ID = tr.Players[0].ID }, ErrDuplicatePlayer},
		{"negative buy in", func(tr *Tournament) { tr.BuyInAmount = -1 }, ErrIncorrectBuyInAmount},
		{"pot overflow", func(tr *Tournament) { tr.BuyInAmount = math.MaxInt64 }, ErrPotOverflow},
		{"empty chips", func(tr *Tournament) { tr.ChipSet = nil }, ErrEmptyChipSet},
		{"invalid chip value", func(tr *Tournament) { tr.ChipSet[0].Value = 0 }, ErrIncorrectChipSet},
		{"invalid chip count", func(tr *Tournament) { tr.ChipSet[0].Count = 0 }, ErrIncorrectChipSet},
		{"zero duration", func(tr *Tournament) { tr.Duration = 0 }, ErrIncorrectDuration},
		{"negative duration", func(tr *Tournament) { tr.Duration = -time.Hour }, ErrIncorrectDuration},
		{"zero level duration", func(tr *Tournament) { tr.LevelDuration = 0 }, ErrIncorrectDuration},
		{"invalid mode", func(tr *Tournament) { tr.SettlementMode = "other" }, ErrInvalidSettlementMode},
		{"invalid style", func(tr *Tournament) { tr.Style = 99 }, ErrInvalidStyle},
		{"empty rebuy id", func(tr *Tournament) { tr.Players[0].Rebuys = []Rebuy{{}} }, ErrInvalidRebuyID},
		{"duplicate rebuy id", func(tr *Tournament) {
			id := uuid.New()
			tr.Players[0].Rebuys = []Rebuy{{ID: id}}
			tr.Players[1].Rebuys = []Rebuy{{ID: id}}
		}, ErrDuplicateRebuy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tournamentFixture()
			tt.change(&input)
			tr, err := NewTournament(input)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, tr)
		})
	}
	t.Run("valid draft", func(t *testing.T) {
		input := tournamentFixture()
		tr, err := NewTournament(input)
		require.NoError(t, err)
		assert.Equal(t, input, trValue(tr))
		assert.Equal(t, int64(4000), tr.Pot())
		assert.False(t, tr.HasResults())
	})
	t.Run("zero buy in draft remains valid", func(t *testing.T) {
		input := tournamentFixture()
		input.BuyInAmount = 0
		tr, err := NewTournament(input)
		require.NoError(t, err)
		assert.Zero(t, tr.Pot())
	})
	t.Run("payments do not affect pot and rebuy can be removed", func(t *testing.T) {
		input := tournamentFixture()
		input.SettlementMode = SettlementBank
		input.Players[0].Rebuys = []Rebuy{{ID: uuid.New(), Paid: false}, {ID: uuid.New(), Paid: true}}
		tr, err := NewTournament(input)
		require.NoError(t, err)
		assert.Equal(t, int64(6000), tr.Pot())
		tr.Players[0].BuyInPaid = true
		tr.Players[0].Rebuys[0].Paid = true
		assert.Equal(t, int64(6000), tr.Pot())
		tr.Players[0].Rebuys = tr.Players[0].Rebuys[:1]
		assert.Equal(t, int64(5000), tr.Pot())
	})
	t.Run("copies nested mutable fields", func(t *testing.T) {
		input := tournamentFixture()
		place := 1
		input.Players[0].Rebuys = []Rebuy{{ID: uuid.New()}}
		input.Results = []Result{{PlayerID: input.Players[0].ID, Place: &place, Prize: 5000}}
		input.StartingStack = StackPlan{Distribution: []ChipDenomination{{Value: 25, Count: 5}}}
		input.BlindStructure = []BlindLevel{{BigBlind: 50}}
		tr, err := NewTournament(input)
		require.NoError(t, err)
		input.Players[0].Name = "changed"
		input.Players[0].Rebuys[0].Paid = true
		input.ChipSet[0].Count = 1
		input.Results[0].Prize = 1
		place = 2
		input.StartingStack.Distribution[0].Count = 1
		input.BlindStructure[0].BigBlind = 1
		assert.Equal(t, "A", tr.Players[0].Name)
		assert.False(t, tr.Players[0].Rebuys[0].Paid)
		assert.Equal(t, 80, tr.ChipSet[0].Count)
		assert.Equal(t, int64(5000), tr.Results[0].Prize)
		assert.Equal(t, 1, *tr.Results[0].Place)
		assert.Equal(t, 5, tr.StartingStack.Distribution[0].Count)
		assert.Equal(t, int64(50), tr.BlindStructure[0].BigBlind)
	})
}

func trValue(tr *Tournament) Tournament { return *tr }

func TestTournamentResults(t *testing.T) {
	place := 1
	tests := []struct {
		name    string
		results func(Tournament) []Result
		wantErr error
	}{
		{"unknown player", func(tr Tournament) []Result { return []Result{{PlayerID: uuid.New(), Place: &place, Prize: 4000}} }, ErrPlayerNotFound},
		{"duplicate player", func(tr Tournament) []Result {
			return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 2000}, {PlayerID: tr.Players[0].ID, Prize: 2000}}
		}, ErrDuplicateResult},
		{"duplicate place", func(tr Tournament) []Result {
			return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 2000}, {PlayerID: tr.Players[1].ID, Place: &place, Prize: 2000}}
		}, ErrDuplicatePayoutPlace},
		{"zero place", func(tr Tournament) []Result {
			p := 0
			return []Result{{PlayerID: tr.Players[0].ID, Place: &p, Prize: 4000}}
		}, ErrInvalidResultPlace},
		{"negative place", func(tr Tournament) []Result {
			p := -1
			return []Result{{PlayerID: tr.Players[0].ID, Place: &p, Prize: 4000}}
		}, ErrInvalidResultPlace},
		{"place exceeds players", func(tr Tournament) []Result {
			p := 5
			return []Result{{PlayerID: tr.Players[0].ID, Place: &p, Prize: 4000}}
		}, ErrInvalidResultPlace},
		{"zero prize", func(tr Tournament) []Result { return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 0}} }, ErrInvalidPrize},
		{"negative prize", func(tr Tournament) []Result { return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: -1}} }, ErrInvalidPrize},
		{"underpayment", func(tr Tournament) []Result {
			return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 3999}}
		}, ErrPrizeSumMismatch},
		{"overpayment", func(tr Tournament) []Result {
			return []Result{{PlayerID: tr.Players[0].ID, Place: &place, Prize: 4001}}
		}, ErrPrizeSumMismatch},
		{"prize overflow", func(tr Tournament) []Result {
			return []Result{{PlayerID: tr.Players[0].ID, Prize: math.MaxInt64}, {PlayerID: tr.Players[1].ID, Prize: math.MaxInt64}}
		}, ErrPrizeSumMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tournamentFixture()
			input.Results = tt.results(input)
			_, err := NewTournament(input)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
	t.Run("manual three way split and paid fourth place", func(t *testing.T) {
		input := tournamentFixture()
		fourth := 4
		input.Results = []Result{
			{PlayerID: input.Players[0].ID, Prize: 1500},
			{PlayerID: input.Players[1].ID, Prize: 1100},
			{PlayerID: input.Players[2].ID, Prize: 900},
			{PlayerID: input.Players[3].ID, Place: &fourth, Prize: 500},
		}
		tr, err := NewTournament(input)
		require.NoError(t, err)
		assert.True(t, tr.HasResults())
		assert.Nil(t, tr.Results[0].Place)
		assert.Equal(t, int64(1500), tr.Results[0].Prize)
		assert.Equal(t, 4, *tr.Results[3].Place)
	})
	t.Run("only prize winners are needed", func(t *testing.T) {
		input := tournamentFixture()
		input.Results = []Result{{PlayerID: input.Players[1].ID, Place: &place, Prize: 4000}}
		tr, err := NewTournament(input)
		require.NoError(t, err)
		require.Len(t, tr.Results, 1)
	})
	t.Run("prize sum cannot overflow even when each prize fits the pot", func(t *testing.T) {
		input := tournamentFixture()
		input.Players = input.Players[:2]
		input.BuyInAmount = math.MaxInt64 / 2
		input.Results = []Result{
			{PlayerID: input.Players[0].ID, Prize: math.MaxInt64/2 + 1},
			{PlayerID: input.Players[1].ID, Prize: math.MaxInt64/2 + 1},
		}
		_, err := NewTournament(input)
		require.ErrorIs(t, err, ErrPrizeSumMismatch)
	})
}
