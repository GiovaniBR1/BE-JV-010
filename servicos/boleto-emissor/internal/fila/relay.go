package fila

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Rodar publica a outbox até o contexto cancelar.
// Se a conexão cai, espera e abre outra, declarando a topologia de novo.
func Rodar(ctx context.Context, url string, caixa Caixa) {
	espera := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := sessao(ctx, url, caixa)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("relay reconectando", "erro", err, "espera", espera)
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

func sessao(ctx context.Context, url string, caixa Caixa) error {
	conn, ch, err := Conectar(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := Declarar(ch); err != nil {
		return err
	}
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("confirm: %w", err)
	}
	// Buffer 1 e uma publicação por vez: cada confirm casa com o publish anterior.
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	fechou := conn.NotifyClose(make(chan *amqp.Error, 1))

	pub := &publicadorConfirmado{ch: ch, confirms: confirms}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-fechou:
			if err == nil {
				return fmt.Errorf("conexão fechou")
			}
			return fmt.Errorf("conexão fechou: %w", err)
		case <-ticker.C:
			if err := EntregarPendentes(ctx, caixa, pub); err != nil {
				return err
			}
		}
	}
}

type publicadorConfirmado struct {
	ch       *amqp.Channel
	confirms <-chan amqp.Confirmation
}

func (p *publicadorConfirmado) Publicar(ctx context.Context, corpo []byte) error {
	err := p.ch.PublishWithContext(ctx, Exchange, RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         corpo,
	})
	if err != nil {
		return fmt.Errorf("publicar: %w", err)
	}
	select {
	case conf := <-p.confirms:
		if !conf.Ack {
			return fmt.Errorf("broker não confirmou a mensagem")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("confirmação demorou demais")
	}
}
