package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr string
}

func NewConfig() (*Config, error) {
	err := godotenv.Load(".env")
	if err != nil {
		return nil, errors.New("error loading .env file")
	}

	addr := os.Getenv("PORT")
	if addr == "" {
		return nil, errors.New("PORT is required")
	}

	// read fomr env
	return &Config{
		Addr: addr,
	}, nil
}
