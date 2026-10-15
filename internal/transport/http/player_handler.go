package http

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/transport/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PlayerService interface {
	CreatePlayer(context.Context, string, time.Time) (domain.Player, error)
	ListPlayers(context.Context, string, int, int) ([]domain.Player, error)
	RenamePlayer(context.Context, uuid.UUID, string, time.Time) (domain.Player, error)
	DeletePlayer(context.Context, uuid.UUID, time.Time) error
}

type PlayerHandler struct {
	service PlayerService
	now     func() time.Time
}

func NewPlayerHandler(service PlayerService) *PlayerHandler {
	return &PlayerHandler{service: service, now: time.Now}
}

type playerNameRequest struct {
	Name *string `json:"name"`
}

func (req *playerNameRequest) UnmarshalJSON(data []byte) error {
	type playerNameBody playerNameRequest
	return decodePlayerJSON(data, (*playerNameBody)(req))
}

type RegistryPlayerResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	IsDeleted bool      `json:"is_deleted"`
}

func registryPlayerResponse(p domain.Player) RegistryPlayerResponse {
	return RegistryPlayerResponse{ID: p.ID, Name: p.Name, IsDeleted: p.DeletedAt != nil}
}

func decodePlayerName(w http.ResponseWriter, r *http.Request) (string, bool) {
	req, ok := decodeLimitedRequest[playerNameRequest](w, r, 4<<10)
	if !ok {
		return "", false
	}
	if req.Name == nil {
		response.WriteError(w, http.StatusBadRequest, "player name is required")
		return "", false
	}
	return *req.Name, true
}

func (h *PlayerHandler) CreatePlayer(w http.ResponseWriter, r *http.Request) {
	name, ok := decodePlayerName(w, r)
	if !ok {
		return
	}
	player, err := h.service.CreatePlayer(r.Context(), name, h.now())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, registryPlayerResponse(player))
}

func (h *PlayerHandler) ListPlayers(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid query")
		return
	}
	q := query["q"]
	if len(q) > 1 {
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
	players, err := h.service.ListPlayers(r.Context(), query.Get("q"), limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	out := make([]RegistryPlayerResponse, 0, len(players))
	for _, p := range players {
		out = append(out, registryPlayerResponse(p))
	}
	response.WriteJSON(w, http.StatusOK, out)
}

func parseRegistryPlayerID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "player_id"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, app.ErrInvalidPlayerID
	}
	return id, nil
}

func (h *PlayerHandler) RenamePlayer(w http.ResponseWriter, r *http.Request) {
	id, err := parseRegistryPlayerID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	name, ok := decodePlayerName(w, r)
	if !ok {
		return
	}
	player, err := h.service.RenamePlayer(r.Context(), id, name, h.now())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, registryPlayerResponse(player))
}

func (h *PlayerHandler) DeletePlayer(w http.ResponseWriter, r *http.Request) {
	id, err := parseRegistryPlayerID(r)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if err := h.service.DeletePlayer(r.Context(), id, h.now()); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
