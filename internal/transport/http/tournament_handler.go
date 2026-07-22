package http

import (
	"context"
	"encoding/json"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type TournamentService interface {
	CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error)
	GetTournamentByID(ctx context.Context, id uuid.UUID, now time.Time) (*domain.Tournament, error)
	DeleteTournament(ctx context.Context, id uuid.UUID) error
	StartTournament(ctx context.Context, id uuid.UUID, now time.Time) error
	PauseTournament(ctx context.Context, id uuid.UUID, now time.Time) error
	ResumeTournament(ctx context.Context, id uuid.UUID, now time.Time) error
	FinishTournament(ctx context.Context, id uuid.UUID, now time.Time) error
	LevelUpTournament(ctx context.Context, id uuid.UUID, now time.Time) error
	AddRebuy(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error
	KnockoutPlayer(ctx context.Context, id uuid.UUID, playerName string, now time.Time) error
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

	tr, err := h.service.GetTournamentByID(r.Context(), id, time.Now())
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

func (h *TournamentHandler) StartTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.StartTournament(r.Context(), id, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) PauseTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.PauseTournament(r.Context(), id, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) ResumeTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.ResumeTournament(r.Context(), id, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) FinishTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.FinishTournament(r.Context(), id, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) LevelUpTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.service.LevelUpTournament(r.Context(), id, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) AddRebuy(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var req PlayerActionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.AddRebuy(r.Context(), id, req.PlayerName, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TournamentHandler) KnockoutPlayer(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var req PlayerActionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.KnockoutPlayer(r.Context(), id, req.PlayerName, time.Now()); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
