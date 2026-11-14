package config

import (
	"fmt"
	"os"
)

// Config vem do ambiente.
// A Parte 2 acrescenta RABBITMQ_URL e a Parte 3, REDIS_URL.
type Config struct {
	Porta       string
	DatabaseURL string
}

func Carregar() (Config, error) {
	porta := os.Getenv("HTTP_PORT")
	if porta == "" {
		porta = "8080"
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return Config{}, fmt.Errorf("DATABASE_URL é obrigatória")
	}
	return Config{Porta: porta, DatabaseURL: url}, nil
}
