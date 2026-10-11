package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinimizeTransfers(t *testing.T) {
	tests := []struct {
		name     string
		balances map[string]int64
		want     []Transfer
	}{
		{
			name:     "winner takes all",
			balances: map[string]int64{"A": 3000, "B": -1000, "C": -1000, "D": -1000},
			want: []Transfer{
				{From: "B", To: "A", Amount: 1000},
				{From: "C", To: "A", Amount: 1000},
				{From: "D", To: "A", Amount: 1000},
			},
		},
		{
			name:     "1 debtor 2 creditors",
			balances: map[string]int64{"A": 1000, "B": 1000, "C": -2000},
			want: []Transfer{
				{From: "C", To: "A", Amount: 1000},
				{From: "C", To: "B", Amount: 1000},
			},
		},
		{
			name:     "mixed with 2 pointers",
			balances: map[string]int64{"A": 2000, "B": 1000, "C": -1500, "D": -1500},
			want: []Transfer{
				{From: "C", To: "A", Amount: 1500},
				{From: "D", To: "A", Amount: 500},
				{From: "D", To: "B", Amount: 1000},
			},
		},
		{
			name:     "skip zero balance",
			balances: map[string]int64{"A": 2000, "B": 0, "C": -1000, "D": -1000},
			want: []Transfer{
				{From: "C", To: "A", Amount: 1000},
				{From: "D", To: "A", Amount: 1000},
			},
		},
		{
			name:     "empty",
			balances: map[string]int64{},
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
	assert.Equal(t, []Transfer{{From: "B", To: "A", Amount: 1000}, {From: "C", To: "A", Amount: 1000}, {From: "D", To: "A", Amount: 1000}}, tr.CalculateTransfers())
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
