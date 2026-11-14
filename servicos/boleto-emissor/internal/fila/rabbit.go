// Pacote fila fala com o RabbitMQ deste serviço.
// A topologia é a mesma no registrador. Os dois declaram de novo:
// o RabbitMQ aceita se o nome e os argumentos forem iguais.
package fila

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GiovaniBR1/BE-JV-010/servicos/boleto-emissor/internal/boleto"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Exchange    = "boletos"
	RoutingKey  = "boleto.emitido"
	Fila        = "boleto.emitido"
	ExchangeDLQ = "boletos.dlx"
	FilaDLQ     = "boleto.emitido.dlq"
)

// Publicador publica BoletoEmitido. Implementa boleto.Publicador.
type Publicador struct {
	ch *amqp.Channel
	// O canal do cliente AMQP não aceita duas publicações ao mesmo tempo.
	mu sync.Mutex
}

func NovoPublicador(ch *amqp.Channel) *Publicador {
	return &Publicador{ch: ch}
}

func (p *Publicador) PublicarBoletoEmitido(ctx context.Context, b boleto.Boleto) error {
	corpo, err := boleto.Codificar(b)
	if err != nil {
		return fmt.Errorf("codificar mensagem: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err = p.ch.PublishWithContext(ctx, Exchange, RoutingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    b.ID,
		Body:         corpo,
	})
	if err != nil {
		return fmt.Errorf("publicar boleto emitido: %w", err)
	}
	return nil
}

var _ boleto.Publicador = (*Publicador)(nil)

// Conectar tenta de novo por uns 30s. O healthcheck do RabbitMQ
// às vezes passa um instante antes da porta AMQP aceitar conexão.
func Conectar(ctx context.Context, url string) (*amqp.Connection, *amqp.Channel, error) {
	var ultimo error
	for tentativa := 1; tentativa <= 15; tentativa++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, err := conn.Channel()
			if err == nil {
				return conn, ch, nil
			}
			conn.Close()
			ultimo = err
		} else {
			ultimo = err
		}
		slog.Warn("rabbitmq indisponível", "tentativa", tentativa, "erro", ultimo)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, nil, fmt.Errorf("conectar no rabbitmq: %w", ultimo)
}

// AvisarSeCair registra no log se a conexão cair fora do desligamento.
func AvisarSeCair(ctx context.Context, conn *amqp.Connection) {
	avisos := conn.NotifyClose(make(chan *amqp.Error, 1))
	go func() {
		err := <-avisos
		if err != nil && ctx.Err() == nil {
			slog.Error("conexão com o rabbitmq caiu", "erro", err)
		}
	}()
}

// Declarar cria exchange, fila, DLQ e o bind. Idempotente.
func Declarar(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(ExchangeDLQ, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("exchange da dlq: %w", err)
	}
	if _, err := ch.QueueDeclare(FilaDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("fila da dlq: %w", err)
	}
	if err := ch.QueueBind(FilaDLQ, RoutingKey, ExchangeDLQ, false, nil); err != nil {
		return fmt.Errorf("bind da dlq: %w", err)
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLQ,
		"x-dead-letter-routing-key": RoutingKey,
	}
	if _, err := ch.QueueDeclare(Fila, true, false, false, false, args); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	if err := ch.QueueBind(Fila, RoutingKey, Exchange, false, nil); err != nil {
		return fmt.Errorf("bind: %w", err)
	}
	return nil
}
