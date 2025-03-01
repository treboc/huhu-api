package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/treboc/huhu-api/internal/handler"
	internalMiddleware "github.com/treboc/huhu-api/internal/middleware"
	"github.com/treboc/huhu-api/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error running server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	adminApiKey := os.Getenv("ADMIN_API_KEY")
	if adminApiKey == "" {
		return fmt.Errorf("ADMIN_API_KEY environment variable not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		return fmt.Errorf("PORT environment variable not set")
	}

	repo, err := repository.NewSQLiteJokeRepository("./jokes.db")
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer repo.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	jokeHandler := handler.NewJokeHandler(repo, logger)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Update in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello from the Jokes API!"))
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	jokeRouter := chi.NewRouter()
	jokeRouter.Get("/", jokeHandler.ListJokes)
	jokeRouter.Get("/random", jokeHandler.GetRandomJoke)
	jokeRouter.Get("/{id}", jokeHandler.GetJoke)

	adminRouter := chi.NewRouter()
	adminRouter.Use(internalMiddleware.AdminAuth(adminApiKey))
	adminRouter.Post("/joke", jokeHandler.CreateJoke)
	adminRouter.Put("/joke/{id}", jokeHandler.UpdateJoke)
	adminRouter.Delete("/joke/{id}", jokeHandler.DeleteJoke)

	apiRouter := chi.NewRouter()
	apiRouter.Mount("/admin", adminRouter)
	apiRouter.Mount("/joke", jokeRouter)

	r.Mount("/api", apiRouter)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting server on port: %s", ":"+port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting server: %v\n", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server exited gracefully")
	return nil
}
