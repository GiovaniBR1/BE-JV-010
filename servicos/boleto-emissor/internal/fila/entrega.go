package fila

import (
	"context"
	"log/slog"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
)

// Caixa é a outbox que o relay drena.
type Caixa interface {
	Pendentes(ctx context.Context, limite int) ([]boleto.EventoPendente, error)
	MarcarEnviado(ctx context.Context, id string) error
	Adiar(ctx context.Context, id string, quando time.Time) error
}

// PublicadorDeEvento manda o JSON já gravado. A implementação de verdade
// espera o publisher confirm do RabbitMQ.
type PublicadorDeEvento interface {
	Publicar(ctx context.Context, corpo []byte) error
}

// Espera é o intervalo antes de tentar de novo uma linha da outbox.
// 1 falha espera 2s, 2 falhas 4s, e o teto é 30s.
func Espera(falhas int) time.Duration {
	if falhas < 1 {
		falhas = 1
	}
	if falhas > 5 {
		falhas = 5
	}
	d := time.Second << falhas
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

// EntregarPendentes publica o que está vencido.
// Se o broker falha, a linha é adiada e o erro sobe para o loop reconectar.
func EntregarPendentes(ctx context.Context, caixa Caixa, pub PublicadorDeEvento) error {
	eventos, err := caixa.Pendentes(ctx, 20)
	if err != nil {
		return err
	}
	for _, ev := range eventos {
		if err := pub.Publicar(ctx, ev.Payload); err != nil {
			quando := time.Now().Add(Espera(ev.Tentativas + 1))
			if adiarErr := caixa.Adiar(ctx, ev.ID, quando); adiarErr != nil {
				slog.Error("adiar outbox", "id", ev.ID, "erro", adiarErr)
			}
			return err
		}
		if err := caixa.MarcarEnviado(ctx, ev.ID); err != nil {
			return err
		}
		slog.Info("outbox enviada", "id", ev.ID)
	}
	return nil
}
