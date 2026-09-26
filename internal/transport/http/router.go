package http

import (
	appfrontend "holdem-tournament-builder/frontend"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *TournamentHandler, authHandler *AuthHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", HealthHandler)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/logout", authHandler.Logout)
	r.With(authHandler.RequireAuth).Get("/auth/me", authHandler.Me)

	r.Post("/tournaments", handler.CreateTournament)
	r.Get("/tournaments/{id}", handler.GetTournamentByID)
	r.Delete("/tournaments/{id}", handler.DeleteTournament)

	r.Post("/tournaments/{id}/start", handler.StartTournament)
	r.Post("/tournaments/{id}/pause", handler.PauseTournament)
	r.Post("/tournaments/{id}/resume", handler.ResumeTournament)
	r.Post("/tournaments/{id}/finish", handler.FinishTournament)
	r.Post("/tournaments/{id}/level-up", handler.LevelUpTournament)
	r.Post("/tournaments/{id}/rebuy", handler.AddRebuy)
	r.Post("/tournaments/{id}/knockout", handler.KnockoutPlayer)

	fileServer := http.FileServer(http.FS(appfrontend.Files))
	r.Handle("/*", fileServer)

	return r
}
