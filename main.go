package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"food-platform/internal/router"
	"food-platform/internal/store"
)

func main() {
	s, err := store.Open(dbPath())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:    ":" + port(),
		Handler: router.New(s),
	}

	// Start the server in the background so main can wait for a shutdown signal.
	go func() {
		log.Printf("Food Platform API listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Block until an interrupt or SIGTERM arrives.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	stop()

	log.Println("shutting down, draining in-flight requests...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}

// port reads the listen port from the PORT env var, defaulting to 8080.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// dbPath reads the SQLite file path from DB_PATH, defaulting to food-platform.db.
func dbPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return "food-platform.db"
}
