package config

import (
	"fmt"
	"os"
)

// Config vem do ambiente.
type Config struct {
	Porta       string
	DatabaseURL string
	RabbitMQURL string
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
	rabbit := os.Getenv("RABBITMQ_URL")
	if rabbit == "" {
		return Config{}, fmt.Errorf("RABBITMQ_URL é obrigatória")
	}
	return Config{Porta: porta, DatabaseURL: url, RabbitMQURL: rabbit}, nil
}
