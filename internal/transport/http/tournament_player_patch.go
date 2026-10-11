package http

import (
	"fmt"
	"net/http"

	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"

	"github.com/google/uuid"
)

type PatchRebuyRequest struct {
	ID   uuid.UUID `json:"id"`
	Paid *bool     `json:"paid"`
}

type PatchTournamentPlayerRequest struct {
	BuyInPaid *bool                `json:"buy_in_paid"`
	Rebuys    *[]PatchRebuyRequest `json:"rebuys"`
}

func (r PatchTournamentPlayerRequest) ToServiceInput() (service.PatchTournamentPlayerInput, error) {
	input := service.PatchTournamentPlayerInput{BuyInPaid: r.BuyInPaid}
	if r.Rebuys != nil {
		rebuys := make([]domain.Rebuy, 0, len(*r.Rebuys))
		for i, rebuy := range *r.Rebuys {
			if rebuy.Paid == nil {
				return service.PatchTournamentPlayerInput{}, fmt.Errorf("rebuys[%d].paid is required and must be boolean", i)
			}
			rebuys = append(rebuys, domain.Rebuy{ID: rebuy.ID, Paid: *rebuy.Paid})
		}
		input.Rebuys = &rebuys
	}
	return input, nil
}

func (h *TournamentHandler) PatchTournamentPlayer(w http.ResponseWriter, r *http.Request) {
	tournamentID, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	playerID, err := parseTournamentPlayerID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	req, ok := decodeRequest[PatchTournamentPlayerRequest](w, r)
	if !ok {
		return
	}
	input, err := req.ToServiceInput()
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	tr, err := h.service.PatchTournamentPlayer(r.Context(), tournamentID, playerID, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeTournament(w, tr)
}
