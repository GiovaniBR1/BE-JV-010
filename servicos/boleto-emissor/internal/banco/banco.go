// Pacote banco abre o Postgres deste serviço e aplica o schema na subida.
// O registrador tem uma cópia pequena deste arquivo de propósito:
// cada serviço muda o próprio banco sem carregar o outro.
package banco

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var schemas embed.FS

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
	entradas, err := schemas.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("ler schema: %w", err)
	}
	nomes := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		if !entrada.IsDir() {
			nomes = append(nomes, entrada.Name())
		}
	}
	sort.Strings(nomes)
	for _, nome := range nomes {
		corpo, err := schemas.ReadFile("sql/" + nome)
		if err != nil {
			return fmt.Errorf("ler %s: %w", nome, err)
		}
		if _, err := pool.Exec(ctx, string(corpo)); err != nil {
			return fmt.Errorf("aplicar %s: %w", nome, err)
		}
	}
	return nil
}
