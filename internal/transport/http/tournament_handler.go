package http

import (
	"context"
	"encoding/json"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
	"net/http"

	"github.com/google/uuid"
)

type TournamentService interface {
	CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error)
	GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	DeleteTournament(ctx context.Context, id uuid.UUID) error
}

type TournamentHandler struct {
	service TournamentService
}

func NewTournamentHandler(service TournamentService) *TournamentHandler {
	return &TournamentHandler{service: service}
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
		handleServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, CreateTournamentResponse{ID: id.String()})
}

func (h *TournamentHandler) GetTournamentByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	tr, err := h.service.GetTournamentByID(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, newTournamentResponse(tr))
}

func (h *TournamentHandler) DeleteTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.DeleteTournament(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
