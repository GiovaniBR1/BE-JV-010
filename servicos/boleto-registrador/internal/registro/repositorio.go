package registro

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repositorio guarda a cópia local. O id do emissor é a chave primária.
type Repositorio struct {
	pool *pgxpool.Pool
}

func NovoRepositorio(pool *pgxpool.Pool) *Repositorio {
	return &Repositorio{pool: pool}
}

// Registrar grava o boleto. Se o id já existe, não muda a linha e devolve
// novo=false. O INSERT é atômico: duas entregas juntas ainda geram uma linha.
func (r *Repositorio) Registrar(ctx context.Context, reg Registro) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO boletos_registrados (
			id, valor_centavos, vencimento, pagador, status, registrado_em
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, reg.ID, reg.ValorCentavos, reg.Vencimento.Format("2006-01-02"), reg.Pagador, reg.Status, reg.RegistradoEm)
	if err != nil {
		return false, fmt.Errorf("salvar registro: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repositorio) BuscarPorID(ctx context.Context, id string) (Registro, bool, error) {
	var (
		reg        Registro
		vencimento time.Time
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, valor_centavos, vencimento, pagador, status, registrado_em
		FROM boletos_registrados
		WHERE id = $1
	`, id).Scan(&reg.ID, &reg.ValorCentavos, &vencimento, &reg.Pagador, &reg.Status, &reg.RegistradoEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registro{}, false, nil
	}
	if err != nil {
		return Registro{}, false, fmt.Errorf("buscar registro: %w", err)
	}
	reg.Vencimento = vencimento
	return reg, true, nil
}
