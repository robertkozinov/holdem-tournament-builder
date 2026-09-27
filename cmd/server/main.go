package main

import (
	"context"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/storage/postgres"
	transporthttp "holdem-tournament-builder/internal/transport/http"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("initialize postgres: %w", err)
	}
	defer pool.Close()

	repo := postgres.NewTournamentRepository(pool)

	tournamentService := service.NewTournamentService(repo)

	tournamentHandler := transporthttp.NewTournamentHandler(tournamentService)

	router := transporthttp.NewRouter(tournamentHandler)

	server := transporthttp.NewServer(router)

	errCh := make(chan error, 1)
	log.Println("server started on :8080")

	go func() {
		errCh <- server.StartServer()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("start server: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("stop server: %w", err)
		}
	}

	return nil
}
