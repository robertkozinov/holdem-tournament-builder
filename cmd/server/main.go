package main

import (
	"context"
	"errors"
	"fmt"
	"holdem-tournament-builder/internal/auth"
	"holdem-tournament-builder/internal/config"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/storage/postgres"
	redisstore "holdem-tournament-builder/internal/storage/redis"
	transporthttp "holdem-tournament-builder/internal/transport/http"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	defer cancelStartup()
	pool, err := postgres.NewPool(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize postgres: %w", err)
	}
	defer pool.Close()

	redisOptions, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("invalid REDIS_URL")
	}
	redisOptions.Protocol = 2
	redisOptions.MaxRetries = -1
	redisOptions.DialTimeout = 2 * time.Second
	redisOptions.ReadTimeout = 2 * time.Second
	redisOptions.WriteTimeout = 2 * time.Second
	redisOptions.ContextTimeoutEnabled = true
	redisOptions.PoolSize = 8
	redisClient := goredis.NewClient(redisOptions)
	defer redisClient.Close()
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		return fmt.Errorf("initialize redis: %w", err)
	}
	cancelStartup()

	repo := postgres.NewTournamentRepository(pool)
	tournamentService := service.NewTournamentService(repo)
	tournamentHandler := transporthttp.NewTournamentHandler(tournamentService)

	users := postgres.NewUserRepository(pool)
	sessions := redisstore.NewSessionStore(redisClient)
	authService := service.NewAuthService(users, sessions, &auth.PasswordHasher{}, &auth.SessionTokenGenerator{}, cfg.SessionLifetime)
	authHandler := transporthttp.NewAuthHandler(authService, transporthttp.CookieOptions{Secure: cfg.CookieSecure}, cfg.RequestSafety)
	playerService := service.NewPlayerService(postgres.NewPlayerRepository(pool))
	playerHandler := transporthttp.NewPlayerHandler(playerService)
	statisticsService := service.NewStatisticsService(postgres.NewStatisticsRepository(pool))
	statisticsHandler := transporthttp.NewStatisticsHandler(statisticsService)
	router := transporthttp.NewRouter(tournamentHandler, authHandler, playerHandler, statisticsHandler)

	server := transporthttp.NewServer(router, cfg.HTTPAddr)

	errCh := make(chan error, 1)
	log.Printf("server started on %s", cfg.HTTPAddr)

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
