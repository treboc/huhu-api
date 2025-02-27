package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/treboc/huhu-api/internal/config"
	"github.com/treboc/huhu-api/internal/handler"
	"github.com/treboc/huhu-api/internal/middleware"
)

func main() {
	err := run()
	if err != nil {
		fmt.Printf("error running server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := config.NewConfig()
	if err != nil {
		fmt.Printf("error reading config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	handler := NewServer(logger, config)

	// Start the server
	srv := &http.Server{
		Addr:         ":" + config.Addr,
		Handler:      handler,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("starting server", slog.Any("PORT", config.Addr))

	err = srv.ListenAndServe()
	if err != nil {
		fmt.Printf("error starting server: %v\n", err)
		os.Exit(1)
	}

	return nil
}

func NewServer(logger *slog.Logger, config *config.Config) http.Handler {
	router := chi.NewRouter()
	addRoutes(router, logger)
	return router
}

func addRoutes(router *chi.Mux, logger *slog.Logger) {
	v1router := chi.NewRouter()
	v1router.Use(middleware.Logger(logger))
	v1router.Get("/healthz", handler.HandleHealthz)
	router.Mount("/v1", v1router)
}
