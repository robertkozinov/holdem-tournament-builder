package domain

import (
	"math/big"

	"github.com/google/uuid"
)

type PlayerStatistics struct {
	PlayerID          uuid.UUID
	Name              string
	TournamentsPlayed int64
	TotalBuyIns       *big.Int
	TotalPrizes       *big.Int
	NetProfit         *big.Int
}
