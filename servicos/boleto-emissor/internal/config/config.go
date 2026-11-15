package config

import (
	"fmt"
	"os"
	"time"
)

// Config vem do ambiente.
type Config struct {
	Porta       string
	DatabaseURL string
	RabbitMQURL string
	RedisURL    string
	CacheTTL    time.Duration
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
	ttl := 5 * time.Minute
	if bruto := os.Getenv("CACHE_TTL"); bruto != "" {
		d, err := time.ParseDuration(bruto)
		if err != nil {
			return Config{}, fmt.Errorf("CACHE_TTL: %w", err)
		}
		ttl = d
	}
	return Config{
		Porta:       porta,
		DatabaseURL: url,
		RabbitMQURL: rabbit,
		RedisURL:    os.Getenv("REDIS_URL"),
		CacheTTL:    ttl,
	}, nil
}
