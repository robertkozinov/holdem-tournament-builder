package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *TournamentHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", HealthHandler)

	r.Post("/tournaments", handler.CreateTournament)
	r.Get("/tournaments", handler.ListTournaments)
	r.Get("/tournaments/{id}", handler.GetTournamentByID)
	r.Put("/tournaments/{id}", handler.SaveTournament)
	r.Patch("/tournaments/{id}/players/{player_id}", handler.PatchTournamentPlayer)
	r.Delete("/tournaments/{id}", handler.DeleteTournament)

	return r
}
