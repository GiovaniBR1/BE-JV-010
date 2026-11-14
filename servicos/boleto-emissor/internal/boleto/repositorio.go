package boleto

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventoPendente é uma linha da outbox que ainda não foi confirmada pelo broker.
type EventoPendente struct {
	ID         string
	Payload    []byte
	Tentativas int
}

// Repositorio grava o boleto e a outbox na mesma transação.
type Repositorio interface {
	Salvar(ctx context.Context, boleto Boleto, evento []byte) error
}

// RepositorioPostgres é a implementação usada quando o serviço sobe.
type RepositorioPostgres struct {
	pool *pgxpool.Pool
}

func NovoRepositorio(pool *pgxpool.Pool) *RepositorioPostgres {
	return &RepositorioPostgres{pool: pool}
}

// Salvar grava o boleto e a mensagem na mesma transação.
// Se o processo cair no meio, ou os dois existem, ou nenhum.
func (r *RepositorioPostgres) Salvar(ctx context.Context, b Boleto, evento []byte) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	defer tx.Rollback(ctx)

	// A data vai como texto AAAA-MM-DD para não depender do fuso da sessão.
	_, err = tx.Exec(ctx, `
		INSERT INTO boletos (id, valor_centavos, vencimento, pagador, status, criado_em)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, b.ID, b.ValorCentavos, b.Vencimento.Format("2006-01-02"), b.Pagador, b.Status, b.CriadoEm)
	if err != nil {
		return fmt.Errorf("salvar boleto: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox (id, boleto_id, payload)
		VALUES ($1, $2, $3)
	`, uuid.NewString(), b.ID, string(evento))
	if err != nil {
		return fmt.Errorf("salvar outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar transação: %w", err)
	}
	return nil
}

func (r *RepositorioPostgres) Pendentes(ctx context.Context, limite int) ([]EventoPendente, error) {
	linhas, err := r.pool.Query(ctx, `
		SELECT id::text, payload, tentativas
		FROM outbox
		WHERE enviado_em IS NULL AND proxima_tentativa_em <= now()
		ORDER BY criado_em
		LIMIT $1
	`, limite)
	if err != nil {
		return nil, fmt.Errorf("listar outbox: %w", err)
	}
	defer linhas.Close()

	var eventos []EventoPendente
	for linhas.Next() {
		var ev EventoPendente
		var payload string
		if err := linhas.Scan(&ev.ID, &payload, &ev.Tentativas); err != nil {
			return nil, fmt.Errorf("ler outbox: %w", err)
		}
		ev.Payload = []byte(payload)
		eventos = append(eventos, ev)
	}
	if err := linhas.Err(); err != nil {
		return nil, fmt.Errorf("ler outbox: %w", err)
	}
	return eventos, nil
}

func (r *RepositorioPostgres) MarcarEnviado(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox SET enviado_em = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("marcar outbox enviada: %w", err)
	}
	return nil
}

func (r *RepositorioPostgres) Adiar(ctx context.Context, id string, quando time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET tentativas = tentativas + 1, proxima_tentativa_em = $2
		WHERE id = $1
	`, id, quando)
	if err != nil {
		return fmt.Errorf("adiar outbox: %w", err)
	}
	return nil
}
