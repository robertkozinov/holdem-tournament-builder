package http

import (
	"holdem-tournament-builder/internal/transport/response"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *TournamentHandler, auth *AuthHandler, players *PlayerHandler, statistics *StatisticsHandler) http.Handler {
	if auth == nil || auth.service == nil {
		panic("auth handler is required")
	}
	if players == nil || players.service == nil {
		panic("player handler is required")
	}
	if statistics == nil || statistics.service == nil {
		panic("statistics handler is required")
	}
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", HealthHandler)
	origin := http.NewCrossOriginProtection()
	origin.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response.WriteError(w, http.StatusForbidden, "cross-origin request forbidden")
	}))
	authRoutes := r.With(noStore, origin.Handler, auth.safety.rateLimit(generalRate))
	authRoutes.With(auth.safety.rateLimit(registerRate), auth.safety.limitHashing).Post("/auth/register", auth.Register)
	authRoutes.With(auth.safety.rateLimit(loginRate), auth.safety.limitHashing).Post("/auth/login", auth.Login)
	authRoutes.Post("/auth/logout", auth.Logout)
	authRoutes.With(auth.authenticate).Get("/auth/me", auth.Me)

	protected := r.With(noStore, origin.Handler, auth.safety.rateLimit(generalRate), auth.authenticate)
	protected.Post("/players", players.CreatePlayer)
	protected.Get("/players", players.ListPlayers)
	protected.Patch("/players/{player_id}", players.RenamePlayer)
	protected.Delete("/players/{player_id}", players.DeletePlayer)
	protected.Get("/statistics/leaderboard", statistics.Leaderboard)
	protected.Post("/tournaments", handler.CreateTournament)
	protected.Get("/tournaments", handler.ListTournaments)
	protected.Get("/tournaments/{id}", handler.GetTournamentByID)
	protected.Put("/tournaments/{id}", handler.SaveTournament)
	protected.Patch("/tournaments/{id}/players/{player_id}", handler.PatchTournamentPlayer)
	protected.Delete("/tournaments/{id}", handler.DeleteTournament)

	return r
}
