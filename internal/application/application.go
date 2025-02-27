package application

import (
	"log/slog"

	"github.com/treboc/huhu-api/internal/config"
)

type Application struct {
	Config *config.Config
	Logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Application {
	return &Application{
		Config: cfg,
		Logger: logger,
	}
}
