package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinimizeTransfers(t *testing.T) {
	a, b, c, d := uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"), uuid.MustParse("00000000-0000-0000-0000-000000000003"), uuid.MustParse("00000000-0000-0000-0000-000000000004")
	tests := []struct {
		name     string
		balances map[uuid.UUID]int64
		want     []Transfer
	}{
		{
			name:     "winner takes all",
			balances: map[uuid.UUID]int64{a: 3000, b: -1000, c: -1000, d: -1000},
			want: []Transfer{
				{From: b, To: a, Amount: 1000},
				{From: c, To: a, Amount: 1000},
				{From: d, To: a, Amount: 1000},
			},
		},
		{
			name:     "1 debtor 2 creditors",
			balances: map[uuid.UUID]int64{a: 1000, b: 1000, c: -2000},
			want: []Transfer{
				{From: c, To: a, Amount: 1000},
				{From: c, To: b, Amount: 1000},
			},
		},
		{
			name:     "mixed with 2 pointers",
			balances: map[uuid.UUID]int64{a: 2000, b: 1000, c: -1500, d: -1500},
			want: []Transfer{
				{From: c, To: a, Amount: 1500},
				{From: d, To: a, Amount: 500},
				{From: d, To: b, Amount: 1000},
			},
		},
		{
			name:     "skip zero balance",
			balances: map[uuid.UUID]int64{a: 2000, b: 0, c: -1000, d: -1000},
			want: []Transfer{
				{From: c, To: a, Amount: 1000},
				{From: d, To: a, Amount: 1000},
			},
		},
		{
			name:     "empty",
			balances: map[uuid.UUID]int64{},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minimizeTransfers(tt.balances)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateTransfers(t *testing.T) {
	input := tournamentFixture()
	input.Players[1].Rebuys = []Rebuy{{ID: uuid.New()}}
	p1, p2 := 1, 2
	input.Results = []Result{{PlayerID: input.Players[0].ID, Place: &p1, Prize: 4000}, {PlayerID: input.Players[1].ID, Place: &p2, Prize: 1000}}
	tr, err := NewTournament(input)
	require.NoError(t, err)
	assert.ElementsMatch(t, []Transfer{{From: input.Players[1].ID, To: input.Players[0].ID, Amount: 1000}, {From: input.Players[2].ID, To: input.Players[0].ID, Amount: 1000}, {From: input.Players[3].ID, To: input.Players[0].ID, Amount: 1000}}, tr.CalculateTransfers())
	t.Run("bank has no transfers", func(t *testing.T) {
		tr, err := NewTournament(input)
		require.NoError(t, err)
		tr.SettlementMode = SettlementBank
		assert.Empty(t, tr.CalculateTransfers())
	})
	t.Run("draft has no transfers", func(t *testing.T) {
		tr, err := NewTournament(input)
		require.NoError(t, err)
		tr.Results = nil
		assert.Empty(t, tr.CalculateTransfers())
	})
	t.Run("manual split settles entire balance", func(t *testing.T) {
		tr, err := NewTournament(input)
		require.NoError(t, err)
		tr.Results = []Result{{PlayerID: tr.Players[0].ID, Prize: 2300}, {PlayerID: tr.Players[1].ID, Prize: 1700}, {PlayerID: tr.Players[2].ID, Prize: 1000}}
		remaining := tr.balances()
		for _, transfer := range tr.CalculateTransfers() {
			remaining[transfer.From] += transfer.Amount
			remaining[transfer.To] -= transfer.Amount
		}
		for _, amount := range remaining {
			assert.Zero(t, amount)
		}
	})
}

func TestCalculateTransfersKeepsSameNamePlayersIndependent(t *testing.T) {
	input := tournamentFixture()
	input.Players = input.Players[:2]
	input.Players[1].Name = input.Players[0].Name
	input.Results = []Result{{PlayerID: input.Players[0].ID, Prize: 2000}}
	transfers := input.CalculateTransfers()
	require.Len(t, transfers, 1)
	assert.Equal(t, input.Players[1].ID, transfers[0].From)
	assert.Equal(t, input.Players[0].ID, transfers[0].To)
	assert.Equal(t, int64(1000), transfers[0].Amount)
}
