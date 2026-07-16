package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"holdem-tournament-builder/internal/storage/postgres"
	transporthttp "holdem-tournament-builder/internal/transport/http"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	router := transporthttp.NewRouter()

	log.Println("server started on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
