package http

import (
	"context"
	"math/big"
	"net/http"
	"net/url"

	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/transport/response"

	"github.com/google/uuid"
)

type StatisticsService interface {
	Leaderboard(context.Context, int, int) ([]domain.PlayerStatistics, error)
}

type StatisticsHandler struct {
	service StatisticsService
}

func NewStatisticsHandler(service StatisticsService) *StatisticsHandler {
	return &StatisticsHandler{service: service}
}

type PlayerStatisticsResponse struct {
	PlayerID          uuid.UUID `json:"player_id"`
	Name              string    `json:"name"`
	TournamentsPlayed int64     `json:"tournaments_played"`
	TotalBuyIns       *big.Int  `json:"total_buy_ins"`
	TotalPrizes       *big.Int  `json:"total_prizes"`
	NetProfit         *big.Int  `json:"net_profit"`
}

func (h *StatisticsHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid query")
		return
	}
	limit, ok := paginationValue(query, "limit", 50, 1, 100)
	if !ok {
		response.WriteError(w, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, ok := paginationValue(query, "offset", 0, 0, int(^uint(0)>>1))
	if !ok {
		response.WriteError(w, http.StatusBadRequest, "invalid offset")
		return
	}
	items, err := h.service.Leaderboard(r.Context(), limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	out := make([]PlayerStatisticsResponse, 0, len(items))
	for _, item := range items {
		out = append(out, PlayerStatisticsResponse{
			PlayerID:          item.PlayerID,
			Name:              item.Name,
			TournamentsPlayed: item.TournamentsPlayed,
			TotalBuyIns:       item.TotalBuyIns,
			TotalPrizes:       item.TotalPrizes,
			NetProfit:         item.NetProfit,
		})
	}
	response.WriteJSON(w, http.StatusOK, out)
}
