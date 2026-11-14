// Pacote banco abre o Postgres deste serviço e aplica o schema na subida.
// Está copiado no emissor de propósito: cada serviço muda o próprio banco.
package banco

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/001_criar_registros.sql
var schema string

func Conectar(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("abrir conexão: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar no postgres: %w", err)
	}
	return pool, nil
}

func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("aplicar schema: %w", err)
	}
	return nil
}
