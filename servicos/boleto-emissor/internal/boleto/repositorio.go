package boleto

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repositorio grava o boleto no Postgres deste serviço.
type Repositorio interface {
	Salvar(ctx context.Context, boleto Boleto) error
}

// RepositorioPostgres é a implementação usada quando o serviço sobe.
type RepositorioPostgres struct {
	pool *pgxpool.Pool
}

func NovoRepositorio(pool *pgxpool.Pool) *RepositorioPostgres {
	return &RepositorioPostgres{pool: pool}
}

func (r *RepositorioPostgres) Salvar(ctx context.Context, b Boleto) error {
	// A data vai como texto AAAA-MM-DD para não depender do fuso da sessão.
	_, err := r.pool.Exec(ctx, `
		INSERT INTO boletos (id, valor_centavos, vencimento, pagador, status, criado_em)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, b.ID, b.ValorCentavos, b.Vencimento.Format("2006-01-02"), b.Pagador, b.Status, b.CriadoEm)
	if err != nil {
		return fmt.Errorf("salvar boleto: %w", err)
	}
	return nil
}
