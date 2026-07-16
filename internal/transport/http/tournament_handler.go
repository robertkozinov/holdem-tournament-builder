package http

import (
	"context"
	"encoding/json"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
	"net/http"

	"github.com/google/uuid"
)

type TournamentService interface {
	CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error)
}

type TournamentHandler struct {
	service TournamentService
}

func NewTournamentHandler(service TournamentService) *TournamentHandler {
	return &TournamentHandler{service: service}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrValidation):
		response.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, app.ErrTournamentNotFound):
		response.WriteError(w, http.StatusNotFound, "tournament not found")
	default:
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *TournamentHandler) CreateTournament(w http.ResponseWriter, r *http.Request) {
	var req CreateTournamentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	input, err := req.ToServiceInput()
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := h.service.CreateTournament(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, CreateTournamentResponse{ID: id.String()})
}
