package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis guarda o JSON do GET /boletos/{id}.
type Redis struct {
	cli *redis.Client
	ttl time.Duration
}

func Novo(url string, ttl time.Duration) (*Redis, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Redis{cli: redis.NewClient(opt), ttl: ttl}, nil
}

func chave(id string) string {
	return "boleto:" + id
}

// Buscar devolve ok=false quando a chave não existe.
// Um erro de conexão sobe para o HTTP decidir seguir sem cache.
func (r *Redis) Buscar(ctx context.Context, id string) ([]byte, bool, error) {
	corpo, err := r.cli.Get(ctx, chave(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return corpo, true, nil
}

func (r *Redis) Guardar(ctx context.Context, id string, corpo []byte) error {
	return r.cli.Set(ctx, chave(id), corpo, r.ttl).Err()
}

func (r *Redis) Apagar(ctx context.Context, id string) error {
	return r.cli.Del(ctx, chave(id)).Err()
}
