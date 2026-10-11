package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type TournamentService interface {
	CreateTournament(ctx context.Context, input service.CreateTournamentInput) (uuid.UUID, error)
	GetTournamentByID(ctx context.Context, id uuid.UUID) (*domain.Tournament, error)
	SaveTournament(ctx context.Context, id uuid.UUID, input service.SaveTournamentInput) (*domain.Tournament, error)
	PatchTournamentPlayer(ctx context.Context, tournamentID, playerID uuid.UUID, input service.PatchTournamentPlayerInput) (*domain.Tournament, error)
	DeleteTournament(ctx context.Context, id uuid.UUID) error
	ListTournaments(ctx context.Context, limit, offset int) ([]*domain.Tournament, error)
}
type TournamentHandler struct{ service TournamentService }

func NewTournamentHandler(service TournamentService) *TournamentHandler {
	return &TournamentHandler{service: service}
}

const maxRequestBytes = 1 << 20

func decodeRequest[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	var req *T
	err := decoder.Decode(&req)
	if err == nil && req != nil {
		var extra json.RawMessage
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return req, true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		response.WriteError(w, http.StatusRequestEntityTooLarge, "request too large")
	} else {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
	}
	return nil, false
}

func (h *TournamentHandler) CreateTournament(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest[CreateTournamentRequest](w, r)
	if !ok {
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
	writeTournament(w, tr)
}
func (h *TournamentHandler) SaveTournament(w http.ResponseWriter, r *http.Request) {
	id, err := parseTournamentID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	req, ok := decodeRequest[SaveTournamentRequest](w, r)
	if !ok {
		return
	}
	input, err := req.ToServiceInput()
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	tr, err := h.service.SaveTournament(r.Context(), id, input)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeTournament(w, tr)
}
func writeTournament(w http.ResponseWriter, tr *domain.Tournament) {
	body, err := newTournamentResponse(tr)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, body)
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
func (h *TournamentHandler) ListTournaments(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid pagination")
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
	tournaments, err := h.service.ListTournaments(r.Context(), limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	out := make([]TournamentResponse, 0, len(tournaments))
	for _, tr := range tournaments {
		body, err := newTournamentResponse(tr)
		if err != nil {
			handleServiceError(w, err)
			return
		}
		out = append(out, body)
	}
	response.WriteJSON(w, http.StatusOK, out)
}
func paginationValue(query url.Values, key string, fallback, minValue, maxValue int) (int, bool) {
	values, present := query[key]
	if !present {
		return fallback, true
	}
	if len(values) != 1 {
		return 0, false
	}
	value, err := strconv.Atoi(values[0])
	return value, err == nil && value >= minValue && value <= maxValue
}
