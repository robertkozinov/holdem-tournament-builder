package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			name:     "chooses minimum number of transfers",
			balances: map[string]int64{"A": -700, "B": -500, "C": 500, "D": 700},
			want: []Transfer{
				{From: "A", To: "D", Amount: 700},
				{From: "B", To: "C", Amount: 500},
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
	tr := &Tournament{
		Contributions: map[string]int64{
			"A": 1000,
			"B": 2000,
			"C": 1000,
			"D": 1000,
		},
		Results: []Result{
			{Name: "A", Place: 1, Prize: 4000},
			{Name: "B", Place: 2, Prize: 1000},
			{Name: "C", Place: 3, Prize: 0},
			{Name: "D", Place: 4, Prize: 0},
		},
	}

	got := tr.CalculateTransfers()

	assert.Equal(t, []Transfer{
		{From: "B", To: "A", Amount: 1000},
		{From: "C", To: "A", Amount: 1000},
		{From: "D", To: "A", Amount: 1000},
	}, got)
}
