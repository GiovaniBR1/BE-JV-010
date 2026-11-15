package fila

import (
	"context"
	"log/slog"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-registrador/internal/registro"
)

// Rodar escuta a fila até o contexto cancelar.
// Se a conexão cai, espera e abre outra, declarando a topologia de novo.
func Rodar(ctx context.Context, url string, gravador registro.Gravador) {
	espera := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := sessao(ctx, url, gravador)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("consumidor reconectando", "erro", err, "espera", espera)
		timer := time.NewTimer(espera)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if espera < 15*time.Second {
			espera *= 2
		}
	}
}

func sessao(ctx context.Context, url string, gravador registro.Gravador) error {
	conn, ch, err := Conectar(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := Declarar(ch); err != nil {
		return err
	}
	return NovoConsumidor(ch, gravador).Iniciar(ctx)
}
