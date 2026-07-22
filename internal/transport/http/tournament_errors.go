package http

import (
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/transport/response"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrValidation):
		response.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, app.ErrInvalidTournamentID):
		response.WriteError(w, http.StatusBadRequest, "invalid tournament id")
	case errors.Is(err, app.ErrTournamentNotFound):
		response.WriteError(w, http.StatusNotFound, "tournament not found")
	case errors.Is(err, domain.ErrPlayerNotFound):
		response.WriteError(w, http.StatusNotFound, "player not found")
	case errors.Is(err, domain.ErrIncorrectStatus),
		errors.Is(err, domain.ErrCantFinish),
		errors.Is(err, domain.ErrMaxBlindLevel),
		errors.Is(err, domain.ErrRebuyNotAllowed),
		errors.Is(err, domain.ErrCantKnockout):
		response.WriteError(w, http.StatusConflict, err.Error())
	default:
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func parseTournamentID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, app.ErrInvalidTournamentID
	}
	return id, nil
}
